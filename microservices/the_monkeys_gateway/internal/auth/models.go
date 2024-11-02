package auth

type LoginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequestBody struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	LoginMethod string `json:"login_method"`
}

type GetEmail struct {
	Email string `json:"email"`
}

type VerifyEmail struct {
	Email string `json:"email"`
}

type UpdatePassword struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type Authorization struct {
	AuthorizationStatus bool   `json:"authorization_status"`
	Error               string `json:"error,omitempty"`
}

type IncorrectReqBody struct {
	Error string `json:"error,omitempty"`
}

type UpdateUsername struct {
	Username string `json:"username"`
}

// Google login response
type GoogleUser struct {
	Email         string `json:"email"`
	FamilyName    string `json:"family_name"`
	GivenName     string `json:"given_name"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}
