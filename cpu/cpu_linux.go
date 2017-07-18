package cpu

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"metrics/kit"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//Информация о использовании ресурсов процессом
type stats struct {
	utime   syscall.Timeval
	stime   syscall.Timeval
	maxRss  int64
	inBlock int64
	ouBlock int64
	volCtx  int64
	invCtx  int64
}

//Информация о процессоре
type InfoStat struct {
	CPU        int32    `json:"cpu"`
	VendorID   string   `json:"vendorId"`
	Family     string   `json:"family"`
	Model      string   `json:"model"`
	Stepping   int32    `json:"stepping"`
	PhysicalID string   `json:"physicalId"`
	CoreID     string   `json:"coreId"`
	Cores      int32    `json:"cores"`
	ModelName  string   `json:"modelName"`
	Mhz        float64  `json:"mhz"`
	CacheSize  int32    `json:"cacheSize"`
	Flags      []string `json:"flags"`
	Microcode  string   `json:"microcode"`
}

// Снятие метрик
type cpuSubscriber struct {
	// Информационный канал
	cpuinfo chan CPUInfo
	// Признак остановки снятия метрик
	interrupt chan struct{}
	// Интервал снятия
	interval time.Duration
	//Расчет нагрузки на CPU
	calcCpu calcCpuPersec
	//Pid процесса, который слушаем
	pid uint64
}

//Расчет загрузки CPU процессом
type calcCpuPersec struct {
	// Хранит значения времен последнего тика
	lticks lasTicksProcess
}

// Последний считанный процессорный тик
type lasTicksProcess struct {
	lastimeProc uint64
	lastimeCPU  uint64
}

// Фабричный метод подписчика метрики памяти
func NewSubscriber(interval time.Duration, chanel chan CPUInfo, pid ...uint64) Subscriber {
	var valpid uint64

	if len(pid) == 0 {
		valpid = uint64(os.Getpid())
	} else {
		valpid = pid[0]
	}
	return &cpuSubscriber{cpuinfo: chanel, interrupt: make(chan struct{}), interval: interval, pid: valpid}
}

// Запуск снятия метрик
func (p *cpuSubscriber) run() {
	var err error
	if len(p.cpuinfo) > 0 {
		<-p.cpuinfo
	}
	ticker := time.NewTicker(p.interval)

	p.calcCpu.lticks.lastimeProc, err = p.calcCpu.getTickProcess(p.pid)
	if err == ErrorPid {
		log.Fatal(err)
		return
	}
	p.calcCpu.lticks.lastimeCPU = cputimetotal()

		defer log.Println("ticker stopped")
		for {
			select {
			case <-ticker.C:
				cpuinfo, err := p.calcCpu.getCPUPersec(p.pid)
				if err != nil {
					log.Fatal(err)
				} else {

				}
				p.cpuinfo <- *cpuinfo
			case <-p.interrupt:
				return
			}
		}
	}
}

//Запуск процесса снятия метрик
func (p *cpuSubscriber) Start() {
	go p.run()
	return
}

// Остановка снятия метрик
func (p *cpuSubscriber) Interrupt() {
	p.interrupt <- struct{}{}
	return
}

//Получение информации о процеччоре
func sysCPUPath(cpu int32, relPath string) string {
	return kit.HostSys(fmt.Sprintf("devices/system/cpu/cpu%d", cpu), relPath)
}

//Информация о ядрах процессора
func finishCPUInfo(c *InfoStat) error {
	var lines []string
	var err error
	var value float64

	if len(c.CoreID) == 0 {
		lines, err = kit.ReadLines(sysCPUPath(c.CPU, "topology/core_id"))
		if err == nil {
			c.CoreID = lines[0]
		}
	}
	//Максимальная рабочая частота прорцессора
	lines, err = kit.ReadLines(sysCPUPath(c.CPU, "cpufreq/cpuinfo_max_freq"))
	//если возникает ошибка при  чтении файла, игнорируем ее
	if err != nil {
		return nil
	}
	value, err = strconv.ParseFloat(lines[0], 64)
	if err != nil {
		return nil
	}
	c.Mhz = value / 1000.0 // value is in kHz
	if c.Mhz > 9999 {
		c.Mhz = c.Mhz / 1000.0 // value in Hz
	}
	return nil
}

//Информация о процессоре в системе
func Info() ([]InfoStat, error) {
	filename := kit.HostProc("cpuinfo")
	lines, _ := kit.ReadLines(filename)

	var ret []InfoStat
	var processorName string

	c := InfoStat{CPU: -1, Cores: 1}
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "Processor":
			processorName = value
		case "processor":
			if c.CPU >= 0 {
				err := finishCPUInfo(&c)
				if err != nil {
					return ret, err
				}
				ret = append(ret, c)
			}
			c = InfoStat{Cores: 1, ModelName: processorName}
			t, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return ret, err
			}
			c.CPU = int32(t)
		case "vendorId", "vendor_id":
			c.VendorID = value
		case "cpu family":
			c.Family = value
		case "model":
			c.Model = value
		case "cache size":
			t, err := strconv.ParseInt(strings.Replace(value, " KB", "", 1), 10, 64)
			if err != nil {
				return ret, err
			}
			c.CacheSize = int32(t)
		case "microcode":
			c.Microcode = value
		}
	}
	if c.CPU >= 0 {
		err := finishCPUInfo(&c)
		if err != nil {
			return ret, err
		}
		ret = append(ret, c)
	}
	return ret, nil
}

//Время на выполнение текущим процессом
func (calc *calcCpuPersec) getTickProcess(pid uint64) (Proct uint64, err error) {
	statPath := kit.HostProc(strconv.Itoa(int(pid)), "stat")
	if statPath == "" {
		return 0, ErrorPid
	}
	contents, err := ioutil.ReadFile(statPath)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(contents))

	i := 1
	for !strings.HasSuffix(fields[i], ")") {
		i++
	}

	/*if err != nil {
		return 0
	}*/
	//Время CPU, которое этот процесс затратил в user mode
	utime, err := strconv.ParseFloat(fields[i+12], 64)
	if err != nil {
		return 0, err
	}
	//Время CPU, которое этот процесс затратил в kernel mode
	stime, err := strconv.ParseFloat(fields[i+13], 64)
	if err != nil {
		return 0, err
	}

	return uint64(stime) + uint64(utime), nil
}

//“общее время работы” процессора, включая время бездействия
func cputimetotal() (cpuTotal uint64) {
	var cputotal uint64 = 0
	var t int = 0
	statPath := kit.HostProc("stat")
	contents, _ := ioutil.ReadFile(statPath)
	lines := strings.Split(string(contents), "\n")

	for _, cputime := range strings.Split(string(lines[0]), " ")[2:] {
		t, _ = strconv.Atoi(cputime)
		cputotal = cputotal + uint64(t)
	}
	return uint64(cputotal)
}

//Получение загрузки процессора
func (calc *calcCpuPersec) getPerfomCPU(proctotal1, proctotal2, cputotal1, cputotal2 uint64) (float64, error) {
	if cputotal2 == cputotal1 {
		return 0.0, errors.New("getPerfomCPU: divide by zero")
	}
	//Формула расчета %CPU
	//res = ((proctotal - pr_proctotal) / (cputotal - pr_cputotal) * 100)
	return float64(float64(proctotal2-proctotal1)/float64(cputotal2-cputotal1)) * 100, nil
}

//Получение нагрзки CPU текущим процессом в %
func (calc *calcCpuPersec) getCPUPersec(pid uint64) (*CPUInfo, error) {
	var ProcTotal1, CPUTotal1 = calc.lticks.lastimeProc, calc.lticks.lastimeCPU
	var err error
	calc.lticks.lastimeProc, err = calc.getTickProcess(pid)
	if err != nil {
		return nil, err
	}
	calc.lticks.lastimeCPU = cputimetotal()
	perfomcpu, err := calc.getPerfomCPU(ProcTotal1, calc.lticks.lastimeProc, CPUTotal1, calc.lticks.lastimeCPU)
	if err != nil {
		return nil, err
	}
	return &CPUInfo{CpuPersec: uint64(kit.Round(perfomcpu, .5, .0))}, nil
}
