// Command echoargs prints what it received: its argv, its working directory
// and a few environment entries. It is used by the demo and by the tests as a
// stand-in for a plugin executable.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Printf("argv0=%s\n", os.Args[0])
	fmt.Printf("argc=%d\n", len(os.Args))
	for i, arg := range os.Args {
		fmt.Printf("argv[%d]=%s\n", i, arg)
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("workdir=<error:%v>\n", err)
	} else {
		fmt.Printf("workdir=%s\n", wd)
	}
	for _, key := range []string{"CI_STEP_ID", "LANG", "PATH", "LD_PRELOAD"} {
		value, ok := os.LookupEnv(key)
		fmt.Printf("env[%s]=%q set=%t\n", key, value, ok)
	}
	fmt.Printf("env_count=%d\n", len(os.Environ()))
	fmt.Println("ok=" + strings.Join(os.Args[1:], "\x1f"))
}
