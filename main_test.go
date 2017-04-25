package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FYI, The bin group is not always 7
func TestGetGIDByGroupName(t *testing.T) {
	result := getGIDByGroupName("bin")
	assert.Equal(t, result, 7)
}

// FYI, The _timezone user is not always 210
func TestGetUIDByUserName(t *testing.T) {
	result := getUIDByUserName("_timezone")
	assert.Equal(t, result, 210)
}

func TestGetUserFile(t *testing.T) {
	err := getUserFile(getWorkingDirectory())
	assert.Equal(t, err, nil)
}
