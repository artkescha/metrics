package kit

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//Получаем значение переменной окружения среды по ключу key
func GetEnv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}
	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
	log.Fatal("invalid switch case")
	return ""
}

//Получение значения переменной среды
func HostProc(combineWith ...string) string {
	return GetEnv("HOST_PROC", "/proc", combineWith...)
}

//Информация о компьютере
func HostSys(combineWith ...string) string {
	return GetEnv("HOST_SYS", "/sys", combineWith...)
}

// Возвращает прочитанную строку файла как []int32.
func ReadInts(filename string) ([]int64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []int64{}, err
	}
	defer f.Close()

	var ret []int64

	r := bufio.NewReader(f)

	//Считываем строку из файла
	line, err := r.ReadString('\n')
	if err != nil {
		return []int64{}, err
	}

	i, err := strconv.ParseInt(strings.Trim(line, "\n"), 10, 32)
	if err != nil {
		return []int64{}, err
	}
	ret = append(ret, i)

	return ret, nil
}

//Файл разбивается на строки
func ReadLines(filename string) ([]string, error) {
	return ReadLinesOffsetN(filename, 0, -1)
}

//Содержимое файла, разбивается на строки
//offset - смещение, с какой строки начинать
//n - количество строк для считывания от смещения
func ReadLinesOffsetN(filename string, offset uint, n int) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}
