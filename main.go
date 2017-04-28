package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
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

// Things to consider that I have not thought about
// - who should be in sudoers file?  maybe those in a group passed in the the user data?

// Read thru groups directory and create new groups.
//  1. Read in json group file.
//  2. Exec out and create group via groupadd.
//  3. As you create the group, loop thru the users in the json file and create a map containing the user as the key and the groups as the value
//	4. Write out each group to leveldb so the next time the code is run, it can determine what has changed

// Read thru users directory and create new users.
//  1. Read in json user file.
//  2. Exec out and create the user via useradd with the correct shell and groups
//	3. Write out each user to leveldb so the next time the code is run, it can determine what has changed
//	4. Create the .ssh directory in the users directory with the correct permissions
//	5. Create the authorized_key file in the ssh directory

// Read thru group files and compare the groups to the groups in leveldb to see if a new group was added or removed
// Read thru user files and compare the users to the users in leveldb to see if a new user was added or removed

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

// Get the groupid by the group name
func getGIDByGroupName(groupName string) int {
	group, err := user.LookupGroup(groupName)
	check(err)

	groupID, err := strconv.Atoi(group.Gid)
	check(err)

	return groupID
}

// Get the userid by the user name
func getUIDByUserName(userName string) int {
	user, err := user.Lookup(userName)
	check(err)

	userID, err := strconv.Atoi(user.Uid)
	check(err)
	return userID
}

// Pipe the output of the curl into tar to create the 2 folders(users/groups)
func getUserGroupFile(workDir string) {
	userURL := os.Getenv("USER_URL")
	if userURL == "" {
		check(errors.New("No User URL passed in"))
	}

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

// Get the WORK_DIR environment variable and create the directory if it doesn't exist
func getWorkingDirectory() string {
	workDir := os.Getenv("WORK_DIR")
	if workDir == "" {
		fmt.Println("WORK_DIR environment variable not set, using /tmp/eau-work")
		workDir = "/tmp/eau-work"
	}

	// if it doesn't exist create it
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		checkWithoutPanic(err)
		os.Mkdir(workDir, 0700)
	}

	return workDir
}

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

// Creates the authorized_keys file in the users ssh directory based on their stored sshkeys
// !!!!!!!! Need to modify to write out multiple keys
func createAuthorizedKeyFile(user ArgoUser, sshDir string) {
	line1 := "# Generated by a Go program sucka\n"
	line2 := "# Local modifications will be overwritten.\n\n"
	line3 := user.SSHkeys[0]

	sshFile := sshDir + "/authorized_keys"

	fmt.Printf("Creating ssh file: %s\n", sshFile)

	d1 := []byte(line1 + line2 + line3)
	err := ioutil.WriteFile(sshFile, d1, 0600)
	check(err)

	fmt.Printf("Changing owner for: %s to %s\n", sshFile, user.ID)

	err = os.Chown(sshFile, getUIDByUserName(user.ID), getGIDByGroupName(user.ID))
	check(err)
}

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

func byteArrayToStringArray(bArray []byte) []string {
	buffer := bytes.NewBuffer(bArray)
	backToStringSlice := []string{}
	gob.NewDecoder(buffer).Decode(&backToStringSlice)
	return backToStringSlice
}

/////////////////////////////////////////////////////////////

// Main
// Pass in -d to remove all users and groups
// Pass in -t to use local files(don't get or remove the gzip'd file)
func main() {
	del := false
	test := false

	if len(os.Args) > 1 {
		if os.Args[1] == "-d" {
			del = true
		}
		if os.Args[1] == "-t" {
			test = true
		}

	}

	workDir := getWorkingDirectory()

	// use level db to track users. needed for deletion.
	db, err := leveldb.OpenFile("/tmp/db", nil)
	check(err)

	defer db.Close()

	if !test {
		// pull down the user group file and uncompress it into the work directory
		getUserGroupFile(workDir)
	}

	// map for users to groups and leveldb comparisons
	mUserGroups := make(map[string][]string)
	mGroup := make(map[string]string)
	mUser := make(map[string]string)

	// Loop through the groups directory creating groups
	// and build the above map to eventually add the users to the appropriate groups
	groupsDir := workDir + "/groups"
	fmt.Printf("Reading directory: %s\n", groupsDir)
	files, _ := ioutil.ReadDir(groupsDir)
	keyPrefix := "group-"
	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		group := getGroupFromFile(file, groupsDir)

		key := keyPrefix + group.ID

		// add group to map
		mGroup[key] = group.ID

		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		if del {
			if data == nil {
				os.Exit(0)
			}

			err = db.Delete([]byte(key), nil)
			check(err)
			groupDelete(group.ID)
		} else {
			newGroup := false
			if data == nil {
				newGroup = true
				fmt.Printf("Creating new group in leveldb with key: %s\n", key)
				err = db.Put([]byte(key), []byte(group.ID), nil)
				check(err)
			} else {
				fmt.Printf("Group %s in leveldb with key: %s already exists.\n", group.ID, key)
			}

			if newGroup {
				groupAdd(group.ID)
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
	// add the users to the appropriate groups,
	// create the .ssh directory and authorized_key file
	usersDir := workDir + "/users"
	fmt.Printf("Reading directory: %s\n", usersDir)
	files, _ = ioutil.ReadDir(usersDir)
	keyPrefix = "user-"

	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		user := getUserFromFile(file, usersDir)

		key := keyPrefix + user.ID

		// add user to map
		mUser[key] = user.ID

		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		if del {
			err = db.Delete([]byte(key), nil)
			check(err)
			userDelete(user.ID)
		} else {
			newUser := false
			if data == nil {
				fmt.Printf("Creating new user in leveldb with key: %s\n", key)
				newUser = true
				stringByte := "\x00" + strings.Join(mUserGroups[user.ID], "\x20\x00") // x20 = space and x00 = null
				err = db.Put([]byte(key), []byte(stringByte), nil)
				check(err)
			} else {
				fmt.Printf("User %s in leveldb with key: %s already exists.\n", user.ID, key)
				abc := byteArrayToStringArray(data)
				fmt.Printf("%v\n", abc)
			}

			if newUser {
				groups := mUserGroups[user.ID]
				userAdd(user, groups)

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
			}
		}
	}

	// Loop thru all the records in leveldb with the group prefix and see if they exist in the mGroup map.
	// Any that exist in leveldb but not in the map should be removed.
	iter := db.NewIterator(util.BytesPrefix([]byte("group-")), nil)
	for iter.Next() {
		// fmt.Printf("%s : %s.\n", string(iter.Key()), string(iter.Value()))
		if mGroup[string(iter.Key())] == "" {
			fmt.Printf("%s is missing.\n", string(iter.Key()))
		}
	}
	iter.Release()
	err = iter.Error()

	// Loop thru all the records in leveldb with the user prefix and see if they exist in the mUser map.
	// Any that exist in leveldb but not in the map should be removed.
	iter = db.NewIterator(util.BytesPrefix([]byte("user-")), nil)
	for iter.Next() {
		// fmt.Printf("%s : %s.\n", string(iter.Key()), string(iter.Value()))
		if mUser[string(iter.Key())] == "" {
			fmt.Printf("%s is missing.\n", string(iter.Key()))
		}
	}
	iter.Release()
	err = iter.Error()

	if !test {
		// Remove working directory from possible prying eyes
		err = os.RemoveAll(workDir)
		check(err)
	}
}
