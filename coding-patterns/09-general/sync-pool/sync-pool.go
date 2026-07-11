package main

import (
	"bytes"
	"fmt"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func formatMessage(parts []string) string {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	for i, p := range parts {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(p)
	}
	return buf.String()
}

func main() {
	result := formatMessage([]string{"alpha", "beta", "gamma"})
	fmt.Println(result)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := formatMessage([]string{"go", "routine", fmt.Sprintf("#%d", n)})
			fmt.Println(r)
		}(i)
	}
	wg.Wait()
}
