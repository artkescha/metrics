package main

import (
	"flag"
	"fmt"
	"git.sedmax.ru/SEDMAX_CORE_V2/metrics/cpu"
	"git.sedmax.ru/SEDMAX_CORE_V2/metrics/mem"
	"git.sedmax.ru/SEDMAX_CORE_V2/metrics/process"
	"time"
)

var pid = flag.Uint64("pid", 0, "example: -pid 2028")

//Нагрузка для тестирования
func load(n int, done chan bool) {
	for i := 1; i <= n; i++ {
		for j := 1; j <= n; j++ {

		}
	}
	done <- true
}

func main() {

	flag.Parse()
	inf, err := cpu.Info()
	if err != nil {

		fmt.Println(err)
	} else {
		fmt.Println("InfoCPU()", inf)
	}

	done := make(chan bool, 1)
	memoryinfo := make(chan mem.MemoryInfo, 1)
	interval := time.Duration(1) * time.Second
	processinfo := make(chan ps.ProcessInfo, 1)
	cpuinfo := make(chan cpu.CPUInfo, 1)
	go load(10000000, done)

	var subcpu cpu.Subscriber
	var subp ps.Subscriber
	var subm mem.Subscriber
	if flag.NFlag() == 0 {
		subcpu = cpu.NewSubscriber(interval, cpuinfo)
		subp = ps.NewSubscriber(interval, processinfo)

		subm = mem.NewSubscriber(interval, memoryinfo)
	} else {
		subcpu = cpu.NewSubscriber(interval, cpuinfo, *pid)
		subp = ps.NewSubscriber(interval, processinfo, *pid)
		subm = mem.NewSubscriber(interval, memoryinfo, *pid)
	}
	subm.Start()
	subcpu.Start()
	subp.Start()
	//subp.Interrupt()
	for {
		fmt.Println("memoryinfo:\n ", <-memoryinfo)
		//fmt.Println("processinfo:\n ", <-processinfo)
		fmt.Println("cpuinfo:\n ", <-cpuinfo)
	}
	<-done

	time.Sleep(10 * time.Second)
}
