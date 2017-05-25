package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var isSudo bool

func init() {
	flag.BoolVar(&isSudo, "issudo", false, "run tests as sudo")

	flag.Parse()
}

// These are fairly high level happy path tests.  I need to write negative tests.

// getGIDByGroupName
func TestGetGIDByGroupNamePass(t *testing.T) {
	group := "sys"
	result, err := getGIDByGroupName(group)
	assert.Nil(t, err)
	assert.Equal(t, result, 3)
}

func TestGetGIDByGroupNameFail(t *testing.T) {
	group := "thiswillfail"
	result, err := getGIDByGroupName(group)
	assert.NotNil(t, err)
	assert.Equal(t, result, -1)
}

// getUIDByUserName
func TestGetUIDByUserNamePass(t *testing.T) {
	user := "root"
	result, err := getUIDByUserName(user)
	assert.Nil(t, err)
	assert.Equal(t, result, 0)
}

func TestGetUIDByUserNameFail(t *testing.T) {
	user := "thiswillfail"
	result, err := getUIDByUserName(user)
	assert.NotNil(t, err)
	assert.Equal(t, result, -1)
}

// userGroupToByteArray
func TestUserGroupByteArrayPass(t *testing.T) {
	userGroupIn := UserGroup{[]string{"a", "b", "c"}, []string{"M", "N", "O"}, "user1", "shell1"}

	byteArray := userGroupToByteArray(userGroupIn)

	userGroupOut := byteArrayToUserGroup(byteArray)

	assert.Equal(t, userGroupOut.Groups, []string{"a", "b", "c"})
	assert.Equal(t, userGroupOut.SSHKeys, []string{"M", "N", "O"})
	assert.Equal(t, userGroupOut.ID, "user1")
	assert.Equal(t, userGroupOut.Shell, "shell1")
}

// createWorkingDirectory
func TestCreateWorkingDirectoryPass(t *testing.T) {
	workDir := "/tmp/justatest"
	err := createWorkingDirectory(workDir)
	assert.Nil(t, err)

	_, err = os.Stat(workDir)
	assert.Nil(t, err)
}

func TestCreateWorkingDirectoryFail(t *testing.T) {
	workDir := "/root/justatest"
	if isSudo {
		err := createWorkingDirectory(workDir)
		assert.Nil(t, err)

		_, err = os.Stat(workDir)
		assert.Nil(t, err)
	} else {
		err := createWorkingDirectory(workDir)
		assert.Nil(t, err)

		_, err = os.Stat(workDir)
		assert.Equal(t, reflect.TypeOf(err), reflect.TypeOf(err.(*os.PathError)))
	}
}

// deleteWorkingDirectory
func TestDeleteWorkingDirectoryPass(t *testing.T) {
	workDir := "/tmp/justatest"
	err := deleteWorkingDirectory(workDir)
	assert.Nil(t, err)

	_, err = os.Stat(workDir)
	assert.Nil(t, err)
}

func TestDeleteWorkingDirectoryFail(t *testing.T) {
	workDir := "/tmp/justatest10"
	err := deleteWorkingDirectory(workDir)

	assert.Equal(t, reflect.TypeOf(err), reflect.TypeOf(err.(*os.PathError)))
	assert.NotNil(t, err)
}

// getGroupFromFile
func TestGetGroupPass(t *testing.T) {
	groupsDir := "./groups"
	files, _ := ioutil.ReadDir(groupsDir)
	result, err := getGroupFromFile(files[0], groupsDir)
	assert.Equal(t, result.ID, "test")
	assert.Nil(t, err)
}

func TestGetGroupFail(t *testing.T) {
	groupsDir := "./"
	files, _ := ioutil.ReadDir(groupsDir)
	result, err := getGroupFromFile(files[0], groupsDir)
	assert.Equal(t, reflect.TypeOf(err), reflect.TypeOf(err.(*os.PathError)))
	assert.Nil(t, result)
}

// getUserFromFile
func TestGetUserPass(t *testing.T) {
	usersDir := "./users"
	files, _ := ioutil.ReadDir(usersDir)
	result, err := getUserFromFile(files[0], usersDir)
	assert.Equal(t, result.ID, "test")
	assert.Nil(t, err)
}

func TestGetUserFail(t *testing.T) {
	usersDir := "./"
	files, _ := ioutil.ReadDir(usersDir)
	result, err := getUserFromFile(files[0], usersDir)
	assert.Equal(t, reflect.TypeOf(err), reflect.TypeOf(err.(*os.PathError)))
	assert.Nil(t, result)
}

// contains
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

func TestDoesNotContainFail(t *testing.T) {
	test := []string{}
	result := contains(test, "")
	assert.Equal(t, result, false)
}

// parseUserKey
func TestParseUserKeyPass(t *testing.T) {
	userKey := "user@12345"
	result, err := parseUserKey(userKey)
	assert.Nil(t, err)
	assert.Equal(t, result, "12345")
}

func TestParseUserKeyFailEmpty(t *testing.T) {
	userKey := ""
	result, err := parseUserKey(userKey)
	assert.Equal(t, err, errors.New("Empty string passed in"))
	assert.Equal(t, result, "")
}

func TestParseUserKeyFailNoAmpersand(t *testing.T) {
	userKey := "test123"
	result, err := parseUserKey(userKey)
	assert.Equal(t, err, errors.New("No ampersand passed in fullkey or multiple ampersands passed"))
	assert.Equal(t, result, "")
}

func TestParseUserKeyFailMultipleAmpersand(t *testing.T) {
	userKey := "test123@test@test1"
	result, err := parseUserKey(userKey)
	assert.Equal(t, err, errors.New("No ampersand passed in fullkey or multiple ampersands passed"))
	assert.Equal(t, result, "")
}

//////////// The following tests run as non sudo and handle cases where they should be run as sudo gracefully. ////////////

// groupAdd
func TestAddGroup(t *testing.T) {
	test := "justatestgroup"
	if isSudo {
		err := groupAdd(test)
		assert.Nil(t, err)
	} else {
		err := groupAdd(test)
		errString := "exit status 10: groupadd: Permission denied.\ngroupadd: cannot lock /etc/group; try again later.\n"
		assert.Equal(t, err, errors.New(errString))
	}
}

func TestAddGroup2(t *testing.T) {
	test := "justatestgroup2"
	if isSudo {
		err := groupAdd(test)
		assert.Nil(t, err)
	} else {
		err := groupAdd(test)
		errString := "exit status 10: groupadd: Permission denied.\ngroupadd: cannot lock /etc/group; try again later.\n"
		assert.Equal(t, err, errors.New(errString))
	}
}

// userAdd
func TestAddUser(t *testing.T) {
	user := ArgoUser{[]string{"testkey"}, "justauserid", "/bin/bash"}
	if isSudo {
		err := userAdd(user, []string{"justatestgroup"})
		assert.Nil(t, err)
	} else {
		err := userAdd(user, []string{"justatestgroup"})
		errString := "exit status 6: useradd: group 'justatestgroup' does not exist\n"
		assert.Equal(t, err, errors.New(errString))
	}
}

// addGroupToUser
func TestAddUserToGroup(t *testing.T) {
	user := "justauserid"
	group := "justatestgroup2"
	if isSudo {
		err := addGroupToUser(user, group)
		assert.Nil(t, err)
	} else {
		err := addGroupToUser(user, group)
		errString := "exit status 6: usermod: group 'justatestgroup2' does not exist\n"
		assert.Equal(t, err, errors.New(errString))
	}
}

// createAuthorizedKeyFile
func TestAddAuthorizedKey(t *testing.T) {
	dir := "/home/justauserid"
	user := ArgoUser{[]string{"testkey"}, "justauserid", "/bin/bash"}
	if isSudo {
		err := createAuthorizedKeyFile(user, dir)
		assert.Nil(t, err)
	} else {
		err := createAuthorizedKeyFile(user, dir)
		errString := "open /home/justauserid/authorized_keys: no such file or directory"
		assert.Equal(t, err.Error(), errString)
	}
}

// addGroupToSudoers
func TestAddGroupToSudoers(t *testing.T) {
	group := "justatestgroup"
	if isSudo {
		err := addGroupToSudoers(group)
		assert.Nil(t, err)
	} else {
		err := addGroupToSudoers(group)
		errString := "open /etc/sudoers.d/argo-justatestgroup: permission denied"
		assert.Equal(t, err.Error(), errString)
	}
}

// deleteSudoersFiles
func TestDeleteSudoersFile(t *testing.T) {
	deleteSudoersFiles()
}

// deleteAuthorizedKeyFile
func TestDeleteAuthorizedKey(t *testing.T) {
	dir := "/home/justauserid"
	user := ArgoUser{[]string{"testkey"}, "justauserid", "/bin/bash"}
	if isSudo {
		err := deleteAuthorizedKeyFile(user, dir)
		assert.Nil(t, err)
	} else {
		err := deleteAuthorizedKeyFile(user, dir)
		errString := "remove /home/justauserid/authorized_keys: no such file or directory"
		assert.Equal(t, err.Error(), errString)
	}
}

// removeGroupFromUser
func TestRemoveUserToGroup(t *testing.T) {
	user := "justauserid"
	group := "justatestgroup2"
	if isSudo {
		err := removeGroupFromUser(user, group)
		assert.Nil(t, err)
	} else {
		err := removeGroupFromUser(user, group)
		errString := "exit status 3: gpasswd: group 'justatestgroup2' does not exist in /etc/group\n"
		assert.Equal(t, err.Error(), errString)
	}
}

// userDelete
func TestDeleteUser(t *testing.T) {
	user := "justauserid"
	if isSudo {
		err := userDelete(user)
		assert.Nil(t, err)
	} else {
		err := userDelete(user)
		errString := "exit status 6: userdel: user 'justauserid' does not exist\n"
		assert.Equal(t, err.Error(), errString)
	}
}

// groupDelete
func TestDeleteGroup(t *testing.T) {
	group := "justatestgroup"
	if isSudo {
		err := groupDelete(group)
		assert.Nil(t, err)
	} else {
		err := groupDelete(group)
		errString := "exit status 6: groupdel: group 'justatestgroup' does not exist\n"
		assert.Equal(t, err.Error(), errString)
	}
}

// Test Delete Group
func TestDeleteGroup2(t *testing.T) {
	group := "justatestgroup2"
	if isSudo {
		err := groupDelete(group)
		assert.Nil(t, err)
	} else {
		err := groupDelete(group)
		errString := "exit status 6: groupdel: group 'justatestgroup2' does not exist\n"
		assert.Equal(t, err.Error(), errString)
	}
}

// adjustSlice

func TestAdjustSliceAddOnly(t *testing.T) {
	existingGroups := []string{"a", "b", "c", "d", "e"}
	addGroups := []string{"f", "g"}
	removeGroups := []string{}

	newGroups := adjustSlice(addGroups, removeGroups, existingGroups)
	assert.Equal(t, len(newGroups), 7)
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g"}, newGroups)
}

func TestAdjustSliceDeleteOnly(t *testing.T) {
	existingGroups := []string{"a", "b", "c", "d", "e"}
	addGroups := []string{}
	removeGroups := []string{"c", "d"}

	newGroups := adjustSlice(addGroups, removeGroups, existingGroups)
	assert.Equal(t, len(newGroups), 3)
	assert.Equal(t, []string{"a", "b", "e"}, newGroups)
}

func TestAdjustSlice(t *testing.T) {
	existingGroups := []string{"a", "b", "c", "d", "e"}
	addGroups := []string{"f", "g"}
	removeGroups := []string{"c", "d"}

	newGroups := adjustSlice(addGroups, removeGroups, existingGroups)
	assert.Equal(t, len(newGroups), 5)
	assert.Equal(t, []string{"a", "b", "e", "f", "g"}, newGroups)
}
