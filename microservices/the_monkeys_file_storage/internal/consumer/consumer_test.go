package consumer

import (
	"fmt"
	"testing"
)

func TestCreateUserFolder(t *testing.T) {
	err := CreateUserFolder("szdkfbvczdfsijhv")
	fmt.Printf("err: %v\n", err)
	if err != nil {
		t.Error(err)
	}
}
