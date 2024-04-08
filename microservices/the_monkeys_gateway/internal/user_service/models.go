package user_service

type ProfileRequestBody struct {
	Id int64 `json:"id"`
}

type UpdateProfile struct {
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	MobileNo    string `json:"mobile,omitempty"`
	About       string `json:"about,omitempty"`
	Instagram   string `json:"instagram,omitempty"`
	Twitter     string `json:"twitter,omitempty"`
	Email       string `json:"email,omitempty"`
}

type GetProfile struct {
	UserName string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type UpdateUserProfile struct {
	UserName      string `json:"username"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	DateOfBirth   string `json:"date_of_birth"`
	Bio           string `json:"bio"`
	Address       string `json:"address"`
	ContactNumber string `json:"contact_number"`
	Email         string `json:"email"`
	IPAddress     string `json:"ip_address"`
	Client        string `json:"client_id"`
	Partial       bool   `json:"partial"`
}
