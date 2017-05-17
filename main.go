package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

//////// All tests were run on a vagrant ubuntu 14.04 image; other os's will be supported in the future ///////////

// VERSION - version
const VERSION = "0.0.3"

////////////////////////////  Supporting Functionality //////////////////////////////

// Generic check function to avoid repeatedly checking for errors and panic after logging error
func check(e error) {
	if e != nil {
		fmt.Println(e.Error())
		panic(e)
	}
}

// Generic check function to avoid repeatedly checking for errors but not panicing
func checkWithoutPanic(e error) {
	if e != nil {
		fmt.Println(e)
	}
}

// Tested
// Get the groupid by the group name
func getGIDByGroupName(groupName string) int {
	group, err := user.LookupGroup(groupName)
	check(err)

	groupID, err := strconv.Atoi(group.Gid)
	check(err)

	return groupID
}

// Tested
// Get the userid by the user name
func getUIDByUserName(userName string) int {
	user, err := user.Lookup(userName)
	check(err)

	userID, err := strconv.Atoi(user.Uid)
	check(err)
	return userID
}

// Not testable
// Pipe the output of the curl into tar to create the 2 folders(users/groups)
func getUserGroupFile(workDir string, userURL string) {
	fmt.Printf("Getting user group url and uncompressing it: %s\n", userURL)

	cmd1 := exec.Command("curl", "-s", userURL)
	cmd2 := exec.Command("tar", "-zxC", workDir)

	pr, pw := io.Pipe()
	cmd1.Stdout = pw
	cmd2.Stdin = pr
	cmd2.Stdout = os.Stdout

	err := cmd1.Start()
	check(err)

	err = cmd2.Start()
	check(err)

	go func() {
		defer pw.Close()
		err = cmd1.Wait()
		check(err)
	}()

	err = cmd2.Wait()
	check(err)
}

// Tested
// Create the working directory if it doesn't exist
func createWorkingDirectory(workDir string) {
	// if it doesn't exist create it
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		checkWithoutPanic(err)
		os.Mkdir(workDir, 0700)
	}
}

// Tested
// Delete the working directory if it doesn't exist
func deleteWorkingDirectory(workDir string) {
	// if it doesn't exist create it
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		checkWithoutPanic(err)
		os.Remove(workDir)
	}
}

// Tested
// Pull the group information from the json file into the ArgoGroup struct
func getGroupFromFile(file os.FileInfo, groupsDir string) ArgoGroup {
	// Read the json file and marshall it into a struct
	var group ArgoGroup
	groupFile := groupsDir + "/" + file.Name()
	fmt.Printf("Reading file: %s\n", groupFile)
	result, err := ioutil.ReadFile(groupFile)
	check(err)

	err = json.Unmarshal(result, &group)
	check(err)

	return group
}

//Tested
// Pull the user information from the json file into a ArgoUser struct
func getUserFromFile(file os.FileInfo, usersDir string) ArgoUser {
	// Read the json file and marshall it into a struct
	var user ArgoUser
	userFile := usersDir + "/" + file.Name()
	fmt.Printf("Reading file: %s\n", userFile)
	result, err := ioutil.ReadFile(userFile)
	check(err)

	err = json.Unmarshal(result, &user)
	check(err)

	return user
}

//Tested
// Creates the authorized_keys file in the users ssh directory based on their stored sshkeys
func createAuthorizedKeyFile(user ArgoUser, sshDir string) {
	fileText := "# Generated by argo-lyte\n"
	fileText += "# Local modifications will be overwritten.\n\n"

	for _, sshKey := range user.SSHkeys {
		fileText += sshKey + "\n"
	}

	sshFile := sshDir + "/authorized_keys"

	fmt.Printf("Creating ssh file: %s\n", sshFile)

	d1 := []byte(fileText)
	err := ioutil.WriteFile(sshFile, d1, 0600)
	check(err)

	fmt.Printf("Changing owner for: %s to %s\n", sshFile, user.ID)

	err = os.Chown(sshFile, getUIDByUserName(user.ID), getGIDByGroupName(user.ID))
	check(err)
}

//Tested
// Delete the authorized_keys file in the users ssh directory
func deleteAuthorizedKeyFile(user ArgoUser, sshDir string) {
	sshFile := sshDir + "/authorized_keys"

	fmt.Printf("Deleting ssh file: %s\n", sshFile)

	err := os.Remove(sshFile)
	check(err)
}

func updateAuthorizedKeyFile(user string, sshkeys []string) {
	argoUser := ArgoUser{sshkeys, user, ""}
	sshDir := "/home/" + user + "/.ssh"
	deleteAuthorizedKeyFile(argoUser, sshDir)
	createAuthorizedKeyFile(argoUser, sshDir)
}

//Tested
// Add the group via the exec command
func groupAdd(groupName string) {
	var cmd *exec.Cmd
	fmt.Printf("Creating group: %v\n", groupName)
	cmd = exec.Command("groupadd", groupName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// Delete the group via the exec command
func groupDelete(groupName string) {
	var cmd *exec.Cmd
	fmt.Printf("Deleting group: %v\n", groupName)
	cmd = exec.Command("groupdel", groupName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// Add a group to a user
func addGroupToUser(user string, group string) {
	var cmd *exec.Cmd
	fmt.Printf("Adding group: %s to user: %s\n", user, group)

	cmd = exec.Command("usermod", "-a", "-G", group, user)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// Remove a group from a user
func removeGroupFromUser(user string, group string) {
	var cmd *exec.Cmd
	fmt.Printf("Deleting group: %s from user: %s\n", user, group)

	cmd = exec.Command("gpasswd", "-d", user, group)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// Add the user via the exec command
func userAdd(user ArgoUser, groups []string) {
	var cmd *exec.Cmd
	fmt.Printf("Creating user: %v and adding to these groups: %v\n", user.ID, groups)

	homeDir := "/home/" + user.ID
	if len(groups) == 0 {
		cmd = exec.Command("useradd", "--shell", user.Shell, "--home", homeDir, "--create-home", user.ID)
	} else {
		commaUsers := strings.Join(groups, ",")
		cmd = exec.Command("useradd", "--shell", user.Shell, "--home", homeDir, "--groups", commaUsers, "--create-home", user.ID)
	}

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// Delete the user via the exec command
func userDelete(userName string) {
	var cmd *exec.Cmd
	fmt.Printf("Deleting user: %v\n", userName)
	cmd = exec.Command("userdel", "--remove", userName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

//Tested
// add a group to the sudoers.d directory to allow group access to sudo
func addGroupToSudoers(group string) {
	fileText := "%" + group + " ALL=(ALL) ALL\n"

	sudoersFile := "/etc/sudoers.d/argo-" + group

	fmt.Printf("Creating sudoers file: %s\n", sudoersFile)

	d1 := []byte(fileText)
	err := ioutil.WriteFile(sudoersFile, d1, 0600)
	check(err)
}

//Tested
// Delete the sudoers file for the test
func deleteSudoersFiles() {
	sudoersDir := "/etc/sudoers.d"
	fmt.Printf("Reading directory: %s\n", sudoersDir)
	files, _ := ioutil.ReadDir(sudoersDir)
	for _, file := range files {
		if !strings.Contains(file.Name(), "argo") {
			continue
		}
		fmt.Printf("Deleting sudoers file in: %s\n", file.Name())

		fullPath := sudoersDir + "/" + file.Name()

		err := os.Remove(fullPath)
		check(err)
	}
}

// Tested
// helper function to deal with byte array conversion
func userGroupToByteArray(userGroup UserGroup) []byte {
	bufferIn := &bytes.Buffer{}
	gob.NewEncoder(bufferIn).Encode(userGroup)
	return []byte(bufferIn.Bytes())
}

// Tested
// helper function to deal with byte array conversion
func byteArrayToUserGroup(bArray []byte) *UserGroup {
	buffer := bytes.NewBuffer(bArray)
	userGroup := new(UserGroup)
	gob.NewDecoder(buffer).Decode(&userGroup)
	return userGroup
}

// Tested
// helper function to pull user out of leveldb key
func parseUserKey(fullKey string) string {
	splitKey := strings.Split(fullKey, "@")
	return splitKey[1]
}

// Tested
// simple array contains
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Tested
// adjust the groups
func adjustSlice(arrayAdd []string, arrayRemove []string, existingArray []string) []string {
	newSlice := make([]string, 0)
	if len(arrayAdd) > 0 || len(arrayRemove) > 0 {
		if len(arrayRemove) > 0 {
			for _, existingItem := range existingArray {
				if !contains(arrayRemove, existingItem) {
					newSlice = append(newSlice, existingItem)
				}
			}
		}

		if len(arrayAdd) > 0 {
			// append the new groups to the slice
			newSlice = append(newSlice, arrayAdd...)
		}
		return newSlice
	}
	return existingArray
}

////////////////////////////  Main Functionality //////////////////////////////

var dbLocation string
var workDirectory string
var userURL string
var sudoGroups string
var delete bool
var retrievefile bool
var removefiles bool

func init() {
	flag.StringVar(&dbLocation, "dblocation", "/tmp/db", "leveldb location")
	flag.StringVar(&workDirectory, "workdirectory", "/tmp/eau-work", "temporary working location")
	flag.StringVar(&userURL, "userurl", "", "argo url to tarred and gzipd user / groups files.")
	flag.StringVar(&sudoGroups, "sudogroups", "", "groups to add as sudo. ex. group1, group2")
	flag.BoolVar(&delete, "delete", false, "deletes groups and users")
	flag.BoolVar(&retrievefile, "retrievefile", true, "retrieves file from remote location")
	flag.BoolVar(&removefiles, "removefiles", true, "removes files from retrieval")
}

// Main
func main() {
	flag.Parse()
	// no args
	if len(os.Args) == 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// arg of help
	if os.Args[1] == "help" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// required item
	if userURL == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	createWorkingDirectory(workDirectory)

	// use level db to track users. needed for deletion and updates
	db, err := leveldb.OpenFile(dbLocation, nil)
	check(err)

	defer db.Close()

	if retrievefile == true {
		// retrieve the user group file and uncompress it into the work directory
		getUserGroupFile(workDirectory, userURL)
	}

	// maps for users to groups and leveldb comparisons
	mUserGroups := make(map[string][]string)
	mUserSSHKeys := make(map[string][]string)
	mGroup := make(map[string]string)
	mUser := make(map[string]string)

	// Loop through the groups directory creating groups
	// and building the above map to eventually add the users to the appropriate groups
	groupsDir := workDirectory + "/groups"
	fmt.Printf("Reading directory: %s\n", groupsDir)
	files, err := ioutil.ReadDir(groupsDir)
	check(err)
	keyPrefix := "group@"
	for _, file := range files {
		// very the file has the json extension
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		// Marshall the json into the ArgoGroup struct
		group := getGroupFromFile(file, groupsDir)

		// build the key for the map and leveldb
		key := keyPrefix + group.ID

		// add group to map
		mGroup[key] = group.ID

		// get group from leveldb
		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		// the delete here makes testing this much easier
		if delete == true {
			if data == nil {
				continue
			}

			fmt.Printf("Deleting group in leveldb with key: %s\n", key)
			err = db.Delete([]byte(key), nil)
			check(err)

			//delete the group from the machine
			groupDelete(group.ID)
		} else {
			// Group is not in leveldb, create it as a new group.
			if data == nil {
				fmt.Printf("Creating new group in leveldb with key: %s\n", key)
				err = db.Put([]byte(key), []byte(group.ID), nil)
				check(err)

				// add the group to the machine
				groupAdd(group.ID)
			} else {
				fmt.Printf("Group %s in leveldb with key: %s already exists.\n", group.ID, key)
			}
		}

		// Build a map of the users to groups
		for _, u := range group.Users {
			if mUserGroups[u] == nil {
				mUserGroups[u] = []string{group.ID}
			} else {
				mUserGroups[u] = append(mUserGroups[u], group.ID)
			}
		}
	}

	// Loop through the users directory creating users,
	// adding the users to the appropriate groups,
	// create the .ssh directory and authorized_key file
	usersDir := workDirectory + "/users"
	fmt.Printf("Reading directory: %s\n", usersDir)
	files, err = ioutil.ReadDir(usersDir)
	check(err)
	keyPrefix = "user@"

	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		// Marshall the json into the ArgoUser struct
		user := getUserFromFile(file, usersDir)

		// build the key for the map and leveldb
		key := keyPrefix + user.ID

		// add user to map
		mUser[key] = user.ID
		mUserSSHKeys[key] = user.SSHkeys

		// get the user from leveldb
		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		// the delete here makes testing this much easier
		if delete == true {
			if data == nil {
				continue
			}

			fmt.Printf("Deleting user in leveldb with key: %s\n", key)
			err = db.Delete([]byte(key), nil)
			check(err)

			//delete the user from the machine
			userDelete(user.ID)
		} else {
			// User is not in leveldb, create it as a new user.
			if data == nil {
				fmt.Printf("Creating new user in leveldb with key: %s\n", key)
				userGroup := UserGroup{Groups: mUserGroups[user.ID], SSHKeys: user.SSHkeys, ID: user.ID, Shell: user.Shell}
				bArray := userGroupToByteArray(userGroup)
				err = db.Put([]byte(key), bArray, nil)
				check(err)

				groups := mUserGroups[user.ID]

				// add the user to the machine
				userAdd(user, groups)

				// !!!!!!!!! May need to add a slight delay here to avoid race condition of creating user and
				// then creating directory for ssh keys using the home directory since I am using an exec in the above code !!!!!!!!!!

				// Create the .ssh directory with only the users accessible permissions then
				// put the ssh key in the directory(which should allow the user to ssh in)
				sshDir := "/home/" + user.ID + "/.ssh"

				fmt.Printf("Creating directory: %s\n", sshDir)

				err = os.Mkdir(sshDir, 0700)
				check(err)

				fmt.Printf("Changing owner for: %s to %s\n", sshDir, user.ID)

				err = os.Chown(sshDir, getUIDByUserName(user.ID), getGIDByGroupName(user.ID))
				check(err)

				if len(user.SSHkeys) > 0 {
					createAuthorizedKeyFile(user, sshDir)
				}

			} else {
				fmt.Printf("User %s with groups: %v in leveldb with key: %s already exists.\n", user.ID, byteArrayToUserGroup(data).Groups, key)
			}
		}
	}

	// Loop thru all the records in leveldb with the user prefix and see if they exist in the mUser map.
	// Any that exist in leveldb but not in the map should be removed.
	// If the user exists in both the map and leveldb,
	// - check the groups to make sure we didn't add or remove the user from a group
	// - check the ssh keys to see if they have chnaged
	iter := db.NewIterator(util.BytesPrefix([]byte("user@")), nil)
	for iter.Next() {
		// if the user is not in our map we created above
		if mUser[string(iter.Key())] == "" {
			fmt.Printf("User %s is missing. Deleting user in leveldb.\n", string(iter.Key()))
			err = db.Delete([]byte(iter.Key()), nil)
			check(err)
			userDelete(parseUserKey(string(iter.Key())))
		} else {
			// User Group functionality

			// Convert groups in leveldb to the existing groups
			existingDBGroups := byteArrayToUserGroup(iter.Value()).Groups

			// Pull groups from map created above
			newMapGroups := mUserGroups[parseUserKey(string(iter.Key()))]

			// check for groups to remove
			groupsToRemove := make([]string, 0)
			for _, existingDBGroup := range existingDBGroups {
				groupExists := contains(newMapGroups, existingDBGroup)
				if !groupExists {
					fmt.Printf("Group %s is being removed from %s.\n", existingDBGroup, parseUserKey(string(iter.Key())))

					// add group to the remove group slice
					groupsToRemove = append(groupsToRemove, existingDBGroup)

					// remove the group from the users profile on the machine
					removeGroupFromUser(parseUserKey(string(iter.Key())), existingDBGroup)
				}
			}
			// check for groups to add
			groupsToAdd := make([]string, 0)
			for _, newMapGroup := range newMapGroups {
				groupExists := contains(existingDBGroups, newMapGroup)
				if !groupExists {
					fmt.Printf("Group %s is being added to %s.\n", newMapGroup, parseUserKey(string(iter.Key())))

					// add group to the add group slice
					groupsToAdd = append(groupsToAdd, newMapGroup)

					// add the group to the users profile on the machine
					addGroupToUser(parseUserKey(string(iter.Key())), newMapGroup)
				}
			}

			// User SSH functionality

			// Convert existing ssh keys in leveldb to the existing SSHKeys
			existingSSHKeys := byteArrayToUserGroup(iter.Value()).SSHKeys

			// Pull ssh keys from map created above
			newMapSSHKeys := mUserSSHKeys[string(iter.Key())]

			// check for groups to remove
			sshKeysToRemove := make([]string, 0)
			for _, existingSSHKey := range existingSSHKeys {
				keyExists := contains(newMapSSHKeys, existingSSHKey)
				if !keyExists {
					fmt.Printf("Key %s is being removed from %s.\n", existingSSHKey, parseUserKey(string(iter.Key())))

					// add group to the remove group slice
					sshKeysToRemove = append(sshKeysToRemove, existingSSHKey)
				}
			}

			sshKeysToAdd := make([]string, 0)
			for _, newMapSSHKey := range newMapSSHKeys {
				keyExists := contains(existingSSHKeys, newMapSSHKey)
				if !keyExists {
					fmt.Printf("Group %s is being added to %s.\n", newMapSSHKey, parseUserKey(string(iter.Key())))

					// add group to the add group slice
					sshKeysToAdd = append(sshKeysToAdd, newMapSSHKey)
				}
			}

			// fmt.Printf("groupsToAdd: %d\n", len(groupsToAdd))
			// fmt.Printf("groupsToRemove: %d\n", len(groupsToRemove))
			// fmt.Printf("sshKeysToAdd: %d\n", len(sshKeysToAdd))
			// fmt.Printf("sshKeysToRemove: %d\n", len(sshKeysToRemove))

			// Update LevelDB

			if len(sshKeysToAdd) > 0 || len(sshKeysToRemove) > 0 || len(groupsToAdd) > 0 || len(groupsToRemove) > 0 {
				// convert the current groups from a byte array to a UserGroup Struct
				userGroup := byteArrayToUserGroup(iter.Value())

				if len(groupsToAdd) > 0 || len(groupsToRemove) > 0 {
					// Adjust the groups

					updatedGroups := adjustSlice(groupsToAdd, groupsToRemove, existingDBGroups)

					fmt.Printf("Current Groups: %v\n", userGroup.Groups)
					fmt.Printf("Updated Groups: %v\n", updatedGroups)

					userGroup.Groups = updatedGroups
				}

				if len(sshKeysToAdd) > 0 || len(sshKeysToRemove) > 0 {
					// Adjust the keys
					updatedSSHKeys := adjustSlice(sshKeysToAdd, sshKeysToRemove, existingSSHKeys)

					fmt.Printf("Current Keyss: %v\n", userGroup.SSHKeys)
					fmt.Printf("Updated Keys: %v\n", updatedSSHKeys)

					// Update the authorized Keys
					updateAuthorizedKeyFile(parseUserKey(string(iter.Key())), updatedSSHKeys)

					userGroup.SSHKeys = updatedSSHKeys
				}

				// convert it back to a byte array
				bArray := userGroupToByteArray(*userGroup)

				// update leveldb with the new groups
				err = db.Put(iter.Key(), bArray, nil)
				check(err)
			}
		}
	}
	iter.Release()
	err = iter.Error()

	// Loop thru all the records in leveldb with the group prefix and see if they exist in the mGroup map.
	// Any that exist in leveldb but not in the map should be removed.
	iter = db.NewIterator(util.BytesPrefix([]byte("group@")), nil)
	for iter.Next() {
		//fmt.Printf("%s\n", mGroup[string(iter.Key())])
		if mGroup[string(iter.Key())] == "" {
			fmt.Printf("Group %s is missing. Deleting group in leveldb.\n", string(iter.Key()))
			err = db.Delete([]byte(iter.Key()), nil)
			check(err)
			groupDelete(string(iter.Value()))
		}
	}
	iter.Release()
	err = iter.Error()

	if len(sudoGroups) > 0 {
		deleteSudoersFiles()
		if delete == false {
			splitSudoGrps := strings.Split(sudoGroups, ",")
			for _, sudoGrp := range splitSudoGrps {
				addGroupToSudoers(sudoGrp)
			}
		}
	}
	if removefiles == true {
		// Remove working directory from possible prying eyes
		err = os.RemoveAll(workDirectory)
		check(err)
	}
}
