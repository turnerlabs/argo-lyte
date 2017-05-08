package main

import (
	"os"
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

// Test the byte array stuff
func TestUserGroupByteArray(t *testing.T) {
	userGroupIn := UserGroup{[]string{"a", "b", "c"}, []string{"M", "N", "O"}, "user1", "shell1"}

	byteArray := userGroupToByteArray(userGroupIn)

	userGroupOut := byteArrayToUserGroup(byteArray)

	assert.Equal(t, userGroupOut.Groups, []string{"a", "b", "c"})
	assert.Equal(t, userGroupOut.SSHKeys, []string{"M", "N", "O"})
	assert.Equal(t, userGroupOut.ID, "user1")
	assert.Equal(t, userGroupOut.Shell, "shell1")
}

// Test creation of working directory
func TestCreateWorkingDirectory(t *testing.T) {
	workDir := "/tmp/justatest"
	createWorkingDirectory(workDir)
	_, err := os.Stat(workDir)
	assert.Nil(t, err)
}

// Test deletion of working directory
func TestDeleteWorkingDirectory(t *testing.T) {
	workDir := "/tmp/justatest"
	deleteWorkingDirectory(workDir)
	_, err := os.Stat(workDir)
	assert.Nil(t, err)
}
