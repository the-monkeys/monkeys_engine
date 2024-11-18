package user_service

import "encoding/json"

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

type TableData struct {
	WithHeadings bool       `json:"withHeadings"`
	Stretched    bool       `json:"stretched"`
	Content      [][]string `json:"content"`
}

type Block struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"` // Use interface{} for polymorphism
	Author []string    `json:"author"`
	Time   int64       `json:"time"`
}

type Blog struct {
	Time   int64   `json:"time"`
	Blocks []Block `json:"blocks"`
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var temp struct {
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Data   json.RawMessage `json:"data"`
		Author []string        `json:"author"`
		Time   int64           `json:"time"`
	}

	// Unmarshal common fields
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Assign common fields
	b.ID = temp.ID
	b.Type = temp.Type
	b.Author = temp.Author
	b.Time = temp.Time

	// Handle polymorphic Data field
	switch temp.Type {
	case "table":
		var tableData TableData
		if err := json.Unmarshal(temp.Data, &tableData); err != nil {
			return err
		}
		b.Data = tableData
	default:
		b.Data = temp.Data // Keep as raw JSON for unknown types
	}

	return nil
}
