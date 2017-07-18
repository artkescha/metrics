package mem

import (
	"errors"
)

//Не найден процесс с таким pid
var (
	ErrorPid = errors.New("Error statpath from Pid!")
)

// Информация о RAM, получаемая библиотекой
type MemoryInfo struct {
	WorkingSetSize       uint64  `json:"workingSetSize"`       //Текущий размер рабочего набора в байтах.
	Percentoftotalmemory float64 `json:"percentoftotalmemory"` //Процент памяти от общего объема RAM
}


type VirtualMemoryStat struct {
	Total uint64 `json:"total"`
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
