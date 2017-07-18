package cpu

import (
	"errors"
)

//Не найден процесс с таким Pid
var (
	ErrorPid = errors.New("Error statpath from Pid!")
)

type CPUInfo struct {
	CpuPersec uint64 //загруженность процессора процессом
}

//Интерфейс слушателя метрики CPU
type Subscriber interface {

	//Снятие метрик
	run()

	//Запуск процесса снятия метрик
	Start()

	//Приостановление снятия метрик
	Interrupt()
}
