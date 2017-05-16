package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGIDByGroupName(t *testing.T) {
	result := getGIDByGroupName("root")
	assert.Equal(t, result, 0)
}

func TestGetUIDByUserName(t *testing.T) {
	result := getUIDByUserName("root")
	assert.Equal(t, result, 0)
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

// Test get group from file
func TestGetGroup(t *testing.T) {
	groupsDir := "./groups"
	files, _ := ioutil.ReadDir(groupsDir)
	result := getGroupFromFile(files[0], groupsDir)
	assert.Equal(t, result.ID, "test")
}

// Test get user from file
func TestGetUser(t *testing.T) {
	usersDir := "./users"
	files, _ := ioutil.ReadDir(usersDir)
	result := getGroupFromFile(files[0], usersDir)
	assert.Equal(t, result.ID, "test")
}

// Test contains function
func TestDoesContain(t *testing.T) {
	test := []string{"hello", "world"}
	result := contains(test, "world")
	assert.Equal(t, result, true)
}

func TestDoesNotContain(t *testing.T) {
	test := []string{"hello", "world"}
	result := contains(test, "zoo")
	assert.Equal(t, result, false)
}

// Test parse user key
func TestParseUserKey(t *testing.T) {
	test := "user@12345"
	result := parseUserKey(test)
	assert.Equal(t, result, "12345")
}

// Must run as sudo

// Test Add Group
func TestAddGroup(t *testing.T) {
	test := "justatestgroup"
	groupAdd(test)
}

func TestAddGroup2(t *testing.T) {
	test := "justatestgroup2"
	groupAdd(test)
}

// Test Add User
func TestAddUser(t *testing.T) {
	user := ArgoUser{[]string{"testkey"}, "justauserid", "/bin/bash"}
	userAdd(user, []string{"justatestgroup"})
}

// Test Add User to Group
func TestAddUserToGroup(t *testing.T) {
	addGroupToUser("justauserid", "justatestgroup2")
}

// Test Remove User from Group
func TestRemoveUserToGroup(t *testing.T) {
	removeGroupFromUser("justauserid", "justatestgroup2")
}

// Test Delete User
func TestDeleteUser(t *testing.T) {
	userDelete("justauserid")
}

// Test Delete Group
func TestDeleteGroup(t *testing.T) {
	test := "justatestgroup"
	groupDelete(test)
}

// Test Delete Group
func TestDeleteGroup2(t *testing.T) {
	test := "justatestgroup2"
	groupDelete(test)
}
