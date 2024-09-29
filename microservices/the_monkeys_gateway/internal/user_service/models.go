package user_service

type ProfileRequestBody struct {
	Id int64 `json:"id"`
}

type GetProfile struct {
	UserName string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type UpdateUserProfile struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	DateOfBirth   string `json:"date_of_birth"`
	Bio           string `json:"bio"`
	Address       string `json:"address"`
	ContactNumber string `json:"contact_number"`
	Twitter       string `json:"twitter"`
	Instagram     string `json:"instagram"`
	LinkedIn      string `json:"linkedin"`
	Github        string `json:"github"`
}

type ReturnMessage struct {
	Message string `json:"message"`
}

type UpdateUserProfileRequest struct {
	Values UpdateUserProfile `json:"values"`
}

type FollowTopic struct {
	Topics []string `json:"topics"`
}

type CoAuthor struct {
	AccountId string `json:"account_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Ip        string `json:"ip"`
	Client    string `json:"client"`
}

type Topics struct {
	Topics   []string `json:"topics"`
	Category string   `json:"category"`
}
