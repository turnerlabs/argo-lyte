package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
)

// Things to consider that I have not thought about
// - who should be in sudoers file?  maybe those in a group passed in the the user data?

// Read thru groups directory and create new groups.
//  1. Read in json group file.
//  2. Exec out and create group via groupadd.
//  3. As you create the group, loop thru the users in the json file and create a map containing the user as the key and the groups as the value

// Read thru users directory and create new users.
//  1. Read in json user file.
//  2. Exec out and create the user via useradd with the correct shell and groups
//	3. Create the .ssh directory in the users directory with the correct permissions
//	4. Create the authorized_key file in the ssh directory

// HELPERS

// Generic check function to avoid repeatedly checking for errors
func check(e error) {
	if e != nil {
		fmt.Println(e)
		panic(e)
	}
}

func checkWithoutPanic(e error) {
	if e != nil {
		fmt.Println(e)
	}
}

func getGIDByGroupName(groupName string) int {
	group, err := user.LookupGroup(groupName)
	check(err)

	groupID, err := strconv.Atoi(group.Gid)
	if err != nil {
		log.Println(err)
		return 0
	}

	return groupID
}

func getUIDByUserName(userName string) int {
	user, err := user.Lookup(userName)
	check(err)

	userID, err := strconv.Atoi(user.Uid)
	if err != nil {
		log.Println(err)
		return 0
	}

	return userID
}

// VARIOUS FUNCTIONS

// Pipe the output of the curl into tar to create the 2 folders(users/groups)
func getUserFile(workDir string) error {
	userURL := os.Getenv("USER_URL")
	if userURL == "" {
		return errors.New("No Master URL passed in")
	}

	fmt.Printf("Getting user url and uncompressing it: %s\n", userURL)

	cmd1 := exec.Command("curl", "-s", userURL)
	cmd2 := exec.Command("tar", "-zxC", workDir)

	pr, pw := io.Pipe()
	cmd1.Stdout = pw
	cmd2.Stdin = pr
	cmd2.Stdout = os.Stdout

	cmd1.Start()
	cmd2.Start()

	go func() {
		defer pw.Close()
		cmd1.Wait()
	}()
	cmd2.Wait()

	return nil

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
		os.Mkdir(workDir, 0700)
	}

	return workDir
}

// Pull the group information from the json file into a group struct so I can do something with it
func getGroupFromFile(file os.FileInfo, groupsDir string) (ArgoGroup, error) {
	// Read the json file and marshall it into a struct
	var group ArgoGroup
	groupFile := groupsDir + "/" + file.Name()
	fmt.Printf("Reading file: %s\n", groupFile)
	result, err := ioutil.ReadFile(groupFile)
	if err != nil {
		return group, err
	}

	err = json.Unmarshal(result, &group)
	if err != nil {
		return group, err
	}

	return group, nil
}

// Pull the user information from the json file into a user struct so I can do something with it
func getUserFromFile(file os.FileInfo, usersDir string) (ArgoUser, error) {
	// Read the json file and marshall it into a struct
	var user ArgoUser
	userFile := usersDir + "/" + file.Name()
	fmt.Printf("Reading file: %s\n", userFile)
	result, err := ioutil.ReadFile(userFile)
	if err != nil {
		return user, err
	}

	err = json.Unmarshal(result, &user)
	if err != nil {
		return user, err
	}

	return user, nil
}

// Creates the authorized_keys file in the users ssh directory based on their stored sshkeys
func createAuthorizedKeyFile(user ArgoUser, sshDir string) error {
	line1 := "# Generated by a Go program sucka\n"
	line2 := "# Local modifications will be overwritten.\n\n"
	line3 := user.SSHkeys[0]

	sshFile := sshDir + "/authorized_keys"

	fmt.Printf("Creating ssh file: %s\n", sshFile)

	d1 := []byte(line1 + line2 + line3)
	err := ioutil.WriteFile(sshFile, d1, 0600)
	if err != nil {
		return err
	}

	fmt.Printf("Changing owner for: %s to %s\n", sshFile, user.ID)

	err = os.Chown(sshFile, getUIDByUserName(user.ID), getGIDByGroupName(user.ID))
	if err != nil {
		return err
	}

	return nil
}

func groupAdd(groupName string) {
	var cmd *exec.Cmd
	fmt.Printf("Creating group: %v\n", groupName)
	cmd = exec.Command("groupadd", groupName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

func groupDelete(groupName string) {
	var cmd *exec.Cmd
	fmt.Printf("Creating group: %v\n", groupName)
	cmd = exec.Command("groupdel", groupName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

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

func userDelete(userName string) {
	var cmd *exec.Cmd
	fmt.Printf("Removing user: %v\n", userName)
	cmd = exec.Command("userdel", "--remove", userName)

	err := cmd.Start()
	check(err)

	err = cmd.Wait()
	check(err)
}

// Main
func main() {
	del := false

	if len(os.Args) > 1 {
		if os.Args[1] == "-d" {
			del = true
		}
	}

	workDir := getWorkingDirectory()

	// use level db to track users. needed for deletion.
	db, err := leveldb.OpenFile("/tmp/db", nil)
	check(err)

	defer db.Close()

	err = getUserFile(workDir)
	check(err)

	// map for users to groups
	m := make(map[string][]string)

	// Spin through the groups data bag directory creating groups and building the above map
	// to eventually add the users to the appropriate groups
	groupsDir := workDir + "/groups"
	fmt.Printf("Reading directory: %s\n", groupsDir)
	files, _ := ioutil.ReadDir(groupsDir)
	newGroup := false
	keyPrefix := "group-"
	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		group, err := getGroupFromFile(file, groupsDir)
		check(err)

		key := keyPrefix + group.ID

		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		if data == nil {
			newGroup = true
			fmt.Printf("Creating new group in leveldb with key: %s\n", key)
			err = db.Put([]byte(key), []byte(group.ID), nil)
			check(err)
		} else {
			fmt.Printf("Group %s in leveldb with key: %s already exists.\n", group.ID, key)
		}

		if del {
			err = db.Delete([]byte(key), nil)
			groupDelete(group.ID)
		} else {
			if newGroup {
				groupAdd(group.ID)
			}
		}

		// Build a map of the users to groups
		for _, u := range group.Users {
			if m[u] == nil {
				m[u] = []string{group.ID}
			} else {
				m[u] = append(m[u], group.ID)
			}
		}
	}

	// Spin through the users data bag directory creating users
	// , add the users to the appropriate groups
	// , create the .ssh directory and authorized_key file
	usersDir := workDir + "/users"
	fmt.Printf("Reading directory: %s\n", usersDir)
	files, _ = ioutil.ReadDir(usersDir)
	newUser := false
	keyPrefix = "user-"

	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		user, err := getUserFromFile(file, usersDir)
		check(err)

		key := keyPrefix + user.ID

		data, err := db.Get([]byte(key), nil)
		checkWithoutPanic(err)

		if data == nil {
			fmt.Printf("Creating new user in leveldb with key: %s\n", key)
			newUser = true
			err = db.Put([]byte(key), []byte(user.ID), nil)
			check(err)
		} else {
			fmt.Printf("User %s in leveldb with key: %s already exists.\n", user.ID, key)
		}

		if del {
			err = db.Delete([]byte(key), nil)
			userDelete(user.ID)
		} else {
			if newUser {
				groups := m[user.ID]
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
					err = createAuthorizedKeyFile(user, sshDir)
					check(err)
				}
			}
		}
	}

	// Remove working directory from possible prying eyes
	err = os.RemoveAll(workDir)
	check(err)
}
