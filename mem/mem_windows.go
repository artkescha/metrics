package mem

import (
	"fmt"
	"git.sedmax.ru/SEDMAX_CORE_V2/metrics/kit"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// Подкючаем системную библиотеку
var (
	libkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGlobalMemoryStatusEx = libkernel32.NewProc("GlobalMemoryStatusEx")
	procGetProcessMemoryInfo = libkernel32.NewProc("K32GetProcessMemoryInfo")
	procGetCurrentProcess    = libkernel32.NewProc("GetCurrentProcess")
	procOpenProcess          = libkernel32.NewProc("OpenProcess")
)

// Глобальные данные о RAM
type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64 // in bytes
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

//Процент от общего количества RAM
func percentFromTotalMemory(totalMemory uint64, usedMemory uint64) float64 {
	return kit.Round((float64(usedMemory*100.0) / float64(totalMemory)), .5, 2)

}

//Получаем общий объем виртуальной памяти на компьютере
func virtualMemory() (uint64, error) {
	var memInfo memoryStatusEx
	memInfo.cbSize = uint32(unsafe.Sizeof(memInfo))
	mem, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if mem == 0 {
		return 0, syscall.GetLastError()
	}

	return memInfo.ullTotalPhys, nil
}

//Расчет использования RAM
type calcMemory struct {
	//Всего виртуальной памяти в системе
	Total uint64 `json:"total"`
}

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
	//Pid процесса, по которому собираем метрики
	pid uint64
	//Ссылка на процесс
	handlecurrentprocess uintptr
}

// Информация о RAM, возвращаемая ситемной библиотекой Windows
type processMemoryCountersEx struct {
	cb                         uint32 `json:"cb"`
	pageFaultCount             uint32 `json:"pageFaultCount"`
	peakWorkingSetSize         uint64 `json:"peakWorkingSetSize"`
	workingSetSize             uint64 `json:"workingSetSize"`
	quotaPeakPagedPoolUsage    uint64 `json:"quotaPeakPagedPoolUsage"`
	quotaPagedPoolUsage        uint64 `json:"quotaPagedPoolUsage"`
	quotaPeakNonPagedPoolUsage uint64 `json:"quotaPeakNonPagedPoolUsage"`
	quotaNonPagedPoolUsage     uint64 `json:"quotaNonPagedPoolUsage"`
	pagefileUsage              uint64 `json:"pagefileUsage"`
	peakPagefileUsage          uint64 `json:"peakPagefileUsage"`
}

// Запуск процесса снятия метрик
func (m *memorySubscriber) run() {
	//при создании слушателя не удалось получить дескриптор процесса
	//нечего делать, выходим
	if m.handlecurrentprocess <= 0 {
		log.Fatal("Error get process handle")
		return
	}
	if len(m.memoryinfo) > 0 {
		<-m.memoryinfo
	}
	ticker := time.NewTicker(m.interval)
	defer log.Println("ticker stopped")
	for {
		select {
		case <-ticker.C:
			memoryinfo, err := m.calc.getProcessMemoryInfo(m.handlecurrentprocess)
			if err != nil {
				log.Fatal(err)
			} else {
				m.memoryinfo <- *memoryinfo
			}
		case <-m.interrupt:
			return
		}
	}
	fmt.Println("EXIT Memory")
}

//Получение ссылки на текущий процесс
func getCurrentProcess() uintptr {
	ret, _, _ := procGetCurrentProcess.Call()

	return uintptr(ret)
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

//Структура расчета memory
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
	return &memorySubscriber{memoryinfo: chanel, interrupt: make(chan struct{}), interval: interval, calc: *newMemCalc(), pid: valpid, handlecurrentprocess: openProcessHandle(valpid)}
}

// Снятие метрики RAM с помощью системной библиотеки Windows
func (calcmem *calcMemory) getProcessMemoryInfo(handlecurrentprocess uintptr) (*MemoryInfo, error) {
	var pmc *processMemoryCountersEx = &processMemoryCountersEx{}
	pmc.cb = uint32(unsafe.Sizeof(*pmc))

	if r1, _, e1 := procGetProcessMemoryInfo.Call(handlecurrentprocess, uintptr(unsafe.Pointer(pmc)),
		uintptr(pmc.cb)); int(r1) == 0 {
		return nil, os.NewSyscallError("GetProcessMemoryInfo", e1)
	} else {
		return &MemoryInfo{WorkingSetSize: pmc.workingSetSize, Percentoftotalmemory: percentFromTotalMemory(calcmem.Total, pmc.workingSetSize)}, nil
	}
}

//Получить handle апрцесса по pid
func openProcessHandle(processId uint64) uintptr {
	handle, _, _ := procOpenProcess.Call(ptr(kit.PROCESS_ALL_ACCESS), ptr(true), ptr(int(processId)))
	return handle
}

//Преобразование указателя к нужному типу
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
