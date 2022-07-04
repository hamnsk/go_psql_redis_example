package user

type User struct {
	Id       int64  `json:"id"`
	NickName string `json:"nickname"`
	FistName string `json:"firstname"`
	LastName string `json:"lastname"`
	Gender   string `json:"gender"`
	Pass     string `json:"pass"`
	Status   uint8  `json:"status"`
}
