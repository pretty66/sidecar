package router

import (
	"errors"
	"strconv"
	"sync"

	"github.com/openmsp/sesdk"
	"github.com/smallnest/weighted"
)

type Distribution struct {
	sync.Mutex
	multiLevel bool
	weight     *weighted.SW
}

type MatchInstanceRoute struct {
	ins   *sesdk.Instance
	route *Route
}

func NewDistribution() *Distribution {
	return &Distribution{}
}

func (d *Distribution) matchInstanceByRoute(info map[string][]*sesdk.Instance, routes []Route) [][]MatchInstanceRoute {
	matchIns := [][]MatchInstanceRoute{}
	for route := range routes {
		insMatch := []MatchInstanceRoute{}
		for k := range info {
			for index := range info[k] {
				insMetadata := info[k][index].Metadata
				isMatch := true
				for rkey, rvalue := range routes[route].Metadata {
					matchVal, ok := insMetadata[rkey]
					if !ok {
						isMatch = false
						break
					}
					if matchVal != rvalue {
						isMatch = false
						break
					}
				}
				if isMatch {
					insMatch = append(insMatch, MatchInstanceRoute{
						ins:   copyInstance(info[k][index]),
						route: &routes[route],
					})
				}
			}
		}

		if len(insMatch) > 0 {
			matchIns = append(matchIns, insMatch)
		}
	}
	return matchIns
}

func (d *Distribution) CreateRule(info map[string][]*sesdk.Instance, routes []Route) {
	// ins := toInstanceSlice(info)
	matchIns := d.matchInstanceByRoute(info, routes)
	if len(matchIns) == 0 {
		return
	}
	d.Lock()
	defer d.Unlock()

	d.setWeight(matchIns)
}

func (d *Distribution) UpdateRule(info map[string][]*sesdk.Instance, routes []Route) {
	// ins := toInstanceSlice(info)
	matchIns := d.matchInstanceByRoute(info, routes)
	d.Lock()
	defer d.Unlock()

	if len(matchIns) == 0 {
		d.weight = nil
		return
	}
	if d.weight == nil {
		d.weight = &weighted.SW{}
	} else {
		d.weight.RemoveAll()
	}
	d.setWeight(matchIns)
}

func (d *Distribution) setWeight(matchIns [][]MatchInstanceRoute) {
	d.weight = &weighted.SW{}
	d.multiLevel = false
	for _, routeMatch := range matchIns {
		if len(routeMatch) > 1 {
			d.multiLevel = true
			wei := &weighted.SW{}
			for k := range routeMatch {
				insWeight, _ := strconv.Atoi(routeMatch[k].ins.Metadata["weight"])
				if insWeight <= 0 {
					insWeight = 10
				}
				wei.Add(routeMatch[k].ins, insWeight)
			}
			d.weight.Add(wei, routeMatch[0].route.Weight)
		} else {
			d.weight.Add(routeMatch[0].ins, routeMatch[0].route.Weight)
		}
	}
}

func (d *Distribution) Weight() (remoteURL string, err error) {
	if d.weight == nil {
		return "", errors.New("no matching target service")
	}
	d.Lock()
	defer d.Unlock()
	var ins *sesdk.Instance
	if d.multiLevel {
		next := d.weight.Next()
		if r, ok := next.(*weighted.SW); ok {
			ins = r.Next().(*sesdk.Instance)
		} else {
			ins = next.(*sesdk.Instance)
		}
	} else {
		ins = d.weight.Next().(*sesdk.Instance)
	}

	return ins.Addrs[0], nil
}

func copyInstance(oi *sesdk.Instance) (ni *sesdk.Instance) {
	ni = new(sesdk.Instance)
	*ni = *oi
	ni.Addrs = make([]string, len(oi.Addrs))
	for i, add := range oi.Addrs {
		ni.Addrs[i] = add
	}
	ni.Metadata = make(map[string]string)
	for k, v := range oi.Metadata {
		ni.Metadata[k] = v
	}
	ni.Attribute = make(map[string]string)
	for k, v := range oi.Attribute {
		ni.Attribute[k] = v
	}
	return
}
