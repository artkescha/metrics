package cpu

//#include <windows.h>
import "C"

import (
	"errors"
	"fmt"
	"git.sedmax.ru/SEDMAX_CORE_V2/metrics/kit"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// include Windows API functions
var (
	libkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetSystemTimes    = libkernel32.NewProc("GetSystemTimes")
	procGetProcessTimes   = libkernel32.NewProc("GetProcessTimes")
	procGetCurrentProcess = libkernel32.NewProc("GetCurrentProcess")
	procOpenProcess       = libkernel32.NewProc("OpenProcess")
)

//Информация о процессоре системный вызов
type Win32_Processor struct {
	LoadPercentage            *uint16
	Family                    uint16
	Manufacturer              string
	Name                      string
	NumberOfLogicalProcessors uint32
	ProcessorID               *string
	Stepping                  *string
	MaxClockSpeed             uint32
}

//Информация о процессоре, через системный вызов
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

//Расчет загрузки CPU процессом
type calcCpuPersec struct {
	// Хранит значения времен последнего тика
	lticks lasTicksProcess
}

//Получение информации о процессоре
func Info() ([]InfoStat, error) {
	var ret []InfoStat
	var dst []Win32_Processor
	q := kit.CreateQuery(&dst, "")
	err := kit.Query(q, &dst)
	if err != nil {
		return ret, err
	}

	var procID string
	for i, l := range dst {
		procID = ""
		if l.ProcessorID != nil {
			procID = *l.ProcessorID
		}

		cpu := InfoStat{
			CPU:        int32(i),
			Family:     fmt.Sprintf("%d", l.Family),
			VendorID:   l.Manufacturer,
			ModelName:  l.Name,
			Cores:      int32(l.NumberOfLogicalProcessors),
			PhysicalID: procID,
			Mhz:        float64(l.MaxClockSpeed),
			Flags:      []string{},
		}
		ret = append(ret, cpu)
	}

	return ret, nil
}

// Последний считанный процессорный тик
type lasTicksProcess struct {
	lastimeProc uint64
	lastimeCPU  uint64
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
	//Ссылка на процесс
	handlecurrentprocess uintptr
	//Pid процесса, который слушаем
	pid uint64
}

// Фабричный метод подписчика метрики памяти
func NewSubscriber(interval time.Duration, chanel chan CPUInfo, pid ...uint64) Subscriber {
	var valpid uint64

	if len(pid) == 0 {
		valpid = uint64(os.Getpid())
	} else {
		valpid = pid[0]
	}
	return &cpuSubscriber{cpuinfo: chanel, interrupt: make(chan struct{}), interval: interval, handlecurrentprocess: openProcessHandle(valpid), pid: valpid}
}

// Запуск снятия метрик
func (p *cpuSubscriber) run() {
	if p.handlecurrentprocess <= 0 {
		log.Fatal("Error get process handle")
		return
	}
	if len(p.cpuinfo) > 0 {
		<-p.cpuinfo
	}
	ticker := time.NewTicker(p.interval)
	p.calcCpu.lticks.lastimeProc, p.calcCpu.lticks.lastimeCPU = p.calcCpu.getTickProcess(p.handlecurrentprocess)

	defer log.Println("ticker stopped")
	for {
		select {
		case <-ticker.C:
			cpuPersec, err := p.calcCpu.getCPUPersec(p.handlecurrentprocess)

			if err != nil {
				log.Fatal(err)
			} else {
				p.cpuinfo <- *cpuPersec
			}
		case <-p.interrupt:
			return
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

// Получение процессорного времени
func (calc *calcCpuPersec) getProcessTimes(hProcess uintptr, pCreationTime, pExitTime, pKernelTime, pUserTime uintptr) bool {
	ret, _, _ := procGetProcessTimes.Call(
		uintptr(hProcess),
		pCreationTime,
		pExitTime,
		pKernelTime,
		pUserTime)

	return ret != 0
}

// Получение системного времени
func (calc *calcCpuPersec) getSystemTimes(lpIdleTime, lpKernelTime, lpUserTime uintptr) bool {
	ret, _, _ := procGetSystemTimes.Call(
		lpIdleTime,
		lpKernelTime,
		lpUserTime)

	return ret != 0
}

// Получение ссылки на текущий процесс
func getCurrentProcess() uintptr {
	ret, _, _ := procGetCurrentProcess.Call()

	return uintptr(ret)
}

// Получение очередного временного тика процесса
func (calc *calcCpuPersec) getTickProcess(handlecurrentprocess uintptr) (TimeProc uint64, TimeCPU uint64) {

	var lpCreationTimepr1, lpExitTimepr1, lpKernelTimepr1, lpUserTimepr1, llpIdleTimecpu1, llpKernelTimecpu1, llpUserTimecpu1 C.FILETIME

	var pCreationTimepr1 = uintptr(unsafe.Pointer(&lpCreationTimepr1))
	var pExitTimepr1 = uintptr(unsafe.Pointer(&lpExitTimepr1))
	var pKernelTimepr1 = uintptr(unsafe.Pointer(&lpKernelTimepr1))
	var pUserTimepr1 = uintptr(unsafe.Pointer(&lpUserTimepr1))

	//Процессорное время
	calc.getProcessTimes(handlecurrentprocess, pCreationTimepr1, pExitTimepr1, pKernelTimepr1, pUserTimepr1)

	var TimeKernelProc1 = calc.filetimetoMillSec(lpKernelTimepr1, 10000)
	var TimeUserProc1 = calc.filetimetoMillSec(lpUserTimepr1, 10000)

	var pIdleTimecpu1 = uintptr(unsafe.Pointer(&llpIdleTimecpu1))
	var pKernelTimecpu1 = uintptr(unsafe.Pointer(&llpKernelTimecpu1))
	var pUserTimecpu1 = uintptr(unsafe.Pointer(&llpUserTimecpu1))

	//Системное время
	calc.getSystemTimes(pIdleTimecpu1, pKernelTimecpu1, pUserTimecpu1)

	var KernelTimesys1 = calc.filetimetoMillSec(llpKernelTimecpu1, 10000)
	var UserTimesys1 = calc.filetimetoMillSec(llpUserTimecpu1, 10000)

	return (TimeKernelProc1 + TimeUserProc1), (KernelTimesys1 + UserTimesys1)
}

//Получение загрузки процессора
func (calc *calcCpuPersec) getPerfomCPU(TimeProc1, TimeProc2, TimeCPU1, TimeCPU2 uint64) (float64, error) {
	if TimeCPU1 == TimeCPU2 {
		return 0.0, errors.New("getPerfomCPU: divide by zero")
	}
	return float64(float64(TimeProc2-TimeProc1)/float64(TimeCPU2-TimeCPU1)) * 100, nil
}

//Переводим из Filetime в MillSec
//Возвращает значкеие в милисекундах
func (calc *calcCpuPersec) filetimetoMillSec(filetime C.FILETIME, mera uint64) uint64 {
	QuadPart := uint64(filetime.dwHighDateTime<<32 | filetime.dwLowDateTime)
	return QuadPart / mera //в миллисекундах

}

//Получение нагрзки CPU текущим процессом в %
func (calc *calcCpuPersec) getCPUPersec(handleprocess uintptr) (*CPUInfo, error) {
	var TimeProc1, TimeCPU1 = calc.lticks.lastimeProc, calc.lticks.lastimeCPU
	calc.lticks.lastimeProc, calc.lticks.lastimeCPU = calc.getTickProcess(handleprocess)
	perfomcpu, err := calc.getPerfomCPU(TimeProc1, calc.lticks.lastimeProc, TimeCPU1, calc.lticks.lastimeCPU)
	if err != nil {
		return nil, err
	}
	return &CPUInfo{CpuPersec: uint64(kit.Round(perfomcpu, .5, .0))}, nil
}

//Получить днскриптор процесса по Pid
func openProcessHandle(processId uint64) uintptr {
	handle, _, _ := procOpenProcess.Call(ptr(kit.PROCESS_ALL_ACCESS), ptr(true), ptr(int(processId)))
	return handle
}

//Приведение к нужному типу указателя
func ptr(val interface{}) uintptr {
	switch val.(type) {
	case string:
		return uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(val.(string))))
	case int:
		return uintptr(val.(int))
	default:
		return uintptr(0)
	}
}
