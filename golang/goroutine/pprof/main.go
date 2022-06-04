package main

import (
	"net/http"
	_ "net/http/pprof"
)

func main() {
	for i := 0; i < 100; i++ {
		go func() {
			select {}
		}()
	}

	_ = http.ListenAndServe("localhost:6060", nil)
}
