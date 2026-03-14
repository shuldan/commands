package commands

// Result — контракт результата выполнения команды.
type Result interface {
	ResultName() string
}

// BaseResult — базовая структура для встраивания в конкретные результаты.
type BaseResult struct {
	Name string `json:"result_name"`
}

// ResultName возвращает имя результата.
func (b BaseResult) ResultName() string {
	return b.Name
}
