package onlinesim

import (
	"testing"
)

func TestGetBalance(t *testing.T) {
	var balance float64

	err := GetBalance(&balance)

	if err != nil {
		t.Error("Some error", err)
	}

	if balance == 0 {
		t.Error("Expected nonzero, got", balance)
	}
}

func TestSetOperationOk(t *testing.T) {
	_, idOp, err := GetNumber(`7`)

	if err != nil {
		t.Error("Some error", err)
	}

	if idOp == 0 {
		t.Error("Expected great than zero, got: ", idOp)
	}

	err = SetOperationOk(idOp, 10)

	if err != nil {
		t.Error("Some error", err)
	}
}
