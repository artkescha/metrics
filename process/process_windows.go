package ps

//#include <windows.h>
import "C"
import (
	"errors"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

//include Windows API functions
var (
	libkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCloseHandle              = libkernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = libkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = libkernel32.NewProc("Process32FirstW")
	procProcess32Next            = libkernel32.NewProc("Process32NextW")
	procGetCurrentProcess        = libkernel32.NewProc("GetCurrentProcess")
	procThread32Next             = libkernel32.NewProc("Thread32Next")
)

//некоторые константы из API-интерфейса Windows
const (
	max_path = 260
)

//информация о процессе, через системный вызов
type processentry32 struct {
	size              uint32
	cntUsage          uint32
	processID         uint32
	defaultHeapID     uintptr
	moduleID          uint32
	cntThreads        uint32
	parentProcessID   uint32
	priorityClassBase int32
	flags             uint32
	exeFile           [max_path]uint16
}

//тип - слушатнль процесса
type processSubscriber struct {
	//информационный канал
	processinfo chan ProcessInfo
	//признак остановки снятия метрик
	interrupt chan struct{}
	//интервал снятия
	interval time.Duration
	//pid процесса
	pid uint64
}

// Снятие метрик
func (p *processSubscriber) run() {
	ticker := time.NewTicker(p.interval)
	defer log.Println("ticker stopped")
	if len(p.processinfo) > 0 {
		<-p.processinfo
	}
	for {
		select {
		case <-ticker.C:
			prinfo, err := getProcessInfo(p.pid)
			if err != nil {
				log.Fatal(err)
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

//Фабричный метод создания слушателя информации о процессе
func NewSubscriber(interval time.Duration, chanel chan ProcessInfo, pid ...uint64) Subscriber {
	var valpid uint64

	if len(pid) == 0 {
		valpid = uint64(os.Getpid())
	} else {
		valpid = pid[0]
	}
	return &processSubscriber{processinfo: chanel, interrupt: make(chan struct{}), interval: interval, pid: valpid}
}

// Все процессы в системе
func processesList() ([]ProcessInfo, error) {
	handle, _ := createToolhelp32Snapshot()

	defer procCloseHandle.Call(handle)

	if handle < 0 {
		return nil, syscall.GetLastError()
	}
	var entry processentry32

	entry.size = uint32(unsafe.Sizeof(entry))
	ret, _ := thread32First(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == false {
		return nil, fmt.Errorf("Error retrieving process info")
	}

	results := make([]ProcessInfo, 0, 50)
	for {
		var procinfo = ProcessInfo{entry.processID, entry.cntThreads}
		results = append(results, procinfo)
		ret := thread32Next(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == false {
			break
		}
	}
	return results, nil
}

//Получение информации о процессе по pid
func getProcessInfo(pid uint64) (*ProcessInfo, error) {
	if pid < 0 {
		return nil, errors.New("Wrong Pid")
	}
	var process ProcessInfo
	processesList, err := processesList()
	if err != nil {
		return nil, err
	} else {

		for _, process = range processesList {
			if uint64(process.ProcessID) == pid {
				return &process, nil
			}
		}
	}
	return nil, errors.New("Pid not Found!")
}

//Снимок всех процессов в системе
func createToolhelp32Snapshot() (handle uintptr, err error) {
	handle, _, _ = procCreateToolhelp32Snapshot.Call(
		0x00000002,
		0)
	if handle <= 0 {
		return handle, syscall.GetLastError()
	}
	return handle, nil
}

//Получение первого процесса из снимка процессов
func thread32First(h, t uintptr) (bool, error) {
	ret, _, _ := procProcess32First.Call(
		h,
		uintptr(unsafe.Pointer(t)))
	if ret == 0 {
		return ret != 0, errors.New("Error get first process in Snapshot. ")
	}
	return ret != 0, nil
}

//Получение следующего процесса из снимка процессов
func thread32Next(h, t uintptr) bool {
	ret, _, _ := procProcess32Next.Call(
		h,
		uintptr(unsafe.Pointer(t)))
	return ret != 0
}

//Получение ссылки на текущий процесс
func getCurrentProcess() uintptr {
	ret, _, _ := procGetCurrentProcess.Call()

	return uintptr(ret)
}
