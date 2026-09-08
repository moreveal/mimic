// Reports the native library used by the pinned gov8 dependency, outside timing.
package main

import (
	"fmt"
	"github.com/maclof/gov8"
)

func main() {
	v, err := gov8.RuntimeVersionString()
	if err != nil {
		panic(err)
	}
	fmt.Println(v)
}
