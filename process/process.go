package ps

import (
	"errors"
)

//Не найден процесс с таким pid
var (
	ErrorPid = errors.New("Error statpath from Pid!")
)

//Информация о процессе
type ProcessInfo struct {
	ProcessID  uint32
	CntThreads uint32
}

//Интерфейс слушателя метрик
type Subscriber interface {

	//Снятие метрик
	run()

	//Запуск процесса снятия метрик
	Start()

	//Остановка процесса снятия метрик
	Interrupt()
}
