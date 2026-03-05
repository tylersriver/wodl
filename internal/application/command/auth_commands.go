package command

type RegisterUserCommand struct {
	Email       string
	Password    string
	DisplayName string
}

type LoginCommand struct {
	Email    string
	Password string
}
