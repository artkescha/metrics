package mem

import (
	"io/ioutil"
	"log"
	"metrics/kit"
	"os"
	"strconv"
	"strings"
	"time"
)

// Подписчик метрики память
type memorySubscriber struct {
	//Информационный канал
	memoryinfo chan MemoryInfo
	//Признак остановки снятия метрик
	interrupt chan struct{}
	//Интервал снятия
	interval time.Duration
	//Информ о памяти в системе
	calc calcMemory
	//Идентификатор процесса, который слушаем
	pid uint64
}

//Расчет использования RAM
type calcMemory struct {
	//Всего виртуальной памяти в системе
	Total uint64 `json:"total"`
}

// Запуск процесса снятия метрик
func (m *memorySubscriber) run() {
	if len(m.memoryinfo) > 0 {
		<-m.memoryinfo
	}

	ticker := time.NewTicker(m.interval)
	defer log.Println("ticker stopped")
	for {
		select {
		case <-ticker.C:
			memoryinfo, err := m.calc.getProcessMemoryInfo(m.pid)
			if err != nil {
				log.Fatal(err)
				return
			} else {
				m.memoryinfo <- *memoryinfo
			}
		case <-m.interrupt:
			return
		}
	}
}

//Запуск процесса снятия метрик
func (m *memorySubscriber) Start() {
	go m.run()
	return
}

// Остановка процесса снятия метрик
func (m *memorySubscriber) Interrupt() {
	m.interrupt <- struct{}{}
	return
}

//Создаем калькуляцию memory
func newMemCalc() *calcMemory {
	virtmem, _ := virtualMemory()
	return &calcMemory{Total: virtmem}
}

// Фабричный метод подписчика метрики памяти
func NewSubscriber(interval time.Duration, chanel chan MemoryInfo, pid ...uint64) Subscriber {
	var valpid uint64

	if len(pid) == 0 {
		valpid = uint64(os.Getpid())
	} else {
		valpid = pid[0]
	}
	return &memorySubscriber{memoryinfo: chanel, interrupt: make(chan struct{}), interval: interval, calc: *newMemCalc(), pid: valpid}
}

//Процент от общего количества RAM
func percentFromTotalMemory(totalMemory uint64, usedMemory uint64) float64 {
	return kit.Round((float64(usedMemory*100.0) / float64(totalMemory)), .5, 2)
}

func (calcmem *calcMemory) getProcessMemoryInfo(pid uint64) (*MemoryInfo, error) {
	statPath := kit.HostProc(strconv.Itoa(int(pid)), "status")
	if statPath == "" {
		return nil, ErrorPid
	}
	contents, err := ioutil.ReadFile(statPath)

	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(contents), "\n")
	var memory MemoryInfo
	for _, line := range lines {
		tabParts := strings.SplitN(line, "\t", 2)
		if len(tabParts) < 2 {
			continue
		}
		value := tabParts[1]
		switch strings.TrimRight(tabParts[0], ":") {
		case "VmRSS":
			value := strings.Trim(value, " kB") // remove last "kB"
			v, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return nil, err
			}
			memory.WorkingSetSize = v
			break
		}
	}
	memory.Percentoftotalmemory = percentFromTotalMemory(calcmem.Total, memory.WorkingSetSize)
	return &memory, nil

}

func virtualMemory() ( /**VirtualMemoryStat*/ uint64, error) {
	filename := kit.HostProc("meminfo")
	lines, _ := kit.ReadLines(filename)
	//ret := &VirtualMemoryStat{}
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		value = strings.Replace(value, " kB", "", -1)

		t, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return t, err
		}
		switch key {
		case "MemTotal":
			return t, nil

		}
	}
	return 0, nil
}
