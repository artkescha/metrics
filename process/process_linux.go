package ps

import (
	//"errors"
	"io/ioutil"
	"log"
	"metrics/kit"
	"os"
	"strconv"
	"strings"
	"time"
)

type processSubscriber struct {
	// Информационный канал
	processinfo chan ProcessInfo
	//Признак остановки снятия метрик
	interrupt chan struct{}
	//Интервал снятия
	interval time.Duration
	//Идентификатор процесса, который слушаем
	pid uint64
}

// Снятие метрик
func (p *processSubscriber) run() {

	if len(p.processinfo) > 0 {
		<-p.processinfo
	}
	ticker := time.NewTicker(p.interval)

	defer log.Println("ticker stopped")
	for {
		select {
		case <-ticker.C:
			prinfo, err := getProcessInfo(p.pid)
			if err != nil {
				log.Fatal(err)
				return
			} else {
				p.processinfo <- *prinfo
			}

		case <-p.interrupt:
			return
		}
	}
}

//Запуск процесса снятия метрик
func (p *processSubscriber) Start() {
	go p.run()
	return
}

//Остановка снятия метрик
func (p *processSubscriber) Interrupt() {
	p.interrupt <- struct{}{}
	return
}

// Фабричный метод подписчика метрики памяти
func NewSubscriber(interval time.Duration, chanel chan ProcessInfo, pid ...uint64) Subscriber {
	var valpid uint64

	if len(pid) == 0 {
		valpid = uint64(os.Getpid())
	} else {
		valpid = pid[0]
	}
	return &processSubscriber{processinfo: chanel, interrupt: make(chan struct{}), interval: interval, pid: valpid}

}

//Информацмя о процессе
func getProcessInfo(pid uint64) (*ProcessInfo, error) {
	statPath := kit.HostProc(strconv.Itoa(int(pid)), "status")
	if statPath == "" {
		return nil, ErrorPid
	}
	contents, err := ioutil.ReadFile(statPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(contents), "\n")
	var process ProcessInfo
	process.ProcessID = uint32(pid)
	for _, line := range lines {
		tabParts := strings.SplitN(line, "\t", 2)
		if len(tabParts) < 2 {
			continue
		}
		value := tabParts[1]
		switch strings.TrimRight(tabParts[0], ":") {
		case "Threads":
			var threads int
			threads, _ = strconv.Atoi(strings.Trim(value, " \t"))
			process.CntThreads = uint32(threads)
			break
		}
	}
	return &process, nil
}
