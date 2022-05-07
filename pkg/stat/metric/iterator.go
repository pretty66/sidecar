/*
 * @Author: gitsrc
 * @Date: 2020-09-10 10:57:55
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-09-10 20:19:47
 * @FilePath: /ServiceCar_CI/pkg/stat/metric/iterator.go
 */
package metric

import "fmt"

// Iterator iterates the buckets within the window.
type Iterator struct {
	count         int
	iteratedCount int
	cur           *Bucket
}

// Next returns true util all of the buckets has been iterated.
func (i *Iterator) Next() bool {
	return i.count != i.iteratedCount
}

// Bucket gets current bucket.
func (i *Iterator) Bucket() Bucket {
	if !(i.Next()) {
		panic(fmt.Errorf("stat/metric: iteration out of range iteratedCount: %d count: %d", i.iteratedCount, i.count))
	}
	bucket := *i.cur
	i.iteratedCount++
	i.cur = i.cur.Next()
	return bucket
}
