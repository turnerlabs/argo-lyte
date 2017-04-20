package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Things to consider that I have not thought about
// - who should be in sudoers file?  maybe those in a group passed in the the user data?
// - should this app execute the curl commands in the code?

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

// Find a GID by Group Name(assuming /etc/group is where it can be found)
func getGIDByGroupName(groupName string) int {
	m := make(map[string]int)
	file, err := os.Open("/etc/group")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// skip the comments
		if scanner.Text()[0] != 35 {
			// file is like bin:*:7
			strArray := strings.Split(scanner.Text(), ":")
			gid, errConv := strconv.Atoi(strArray[2])
			check(errConv)
			m[strArray[0]] = gid
		}
	}

	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return m[groupName]
}

// Find a UID by User Name(assuming /etc/passwd is where it can be found)
func getUIDByUserName(userName string) int {
	m := make(map[string]int)
	file, err := os.Open("/etc/passwd")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// skip the comments
		if scanner.Text()[0] != 35 {
			// file is like _warmd:*:224:224:Warm Daemon:/var/empty:/usr/bin/false
			strArray := strings.Split(scanner.Text(), ":")
			uid, errConv := strconv.Atoi(strArray[2])
			check(errConv)
			m[strArray[0]] = uid
		}
	}

	if err = scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return m[userName]
}

// VARIOUS FUNCTIONS

func getMasterFile(workDir string) error {
	masterURL := os.Getenv("MASTER_URL")
	if masterURL == "" {
		return errors.New("No Master URL passed in")
	}

	fmt.Printf("Getting master url and uncompressing it: %s\n", masterURL)
	cmd1 := exec.Command("curl", "-s", masterURL)
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

func getUserFile(workDir string) error {
	userURL := os.Getenv("USER_URL")
	if userURL == "" {
		return errors.New("No Master URL passed in")
	}

	fmt.Printf("Getting user url and uncompressing it: %s\n", userURL)

	userLocation := workDir + "/data_bags"
	cmd1 := exec.Command("curl", "-s", userURL)
	cmd2 := exec.Command("tar", "-zxC", userLocation)

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
func getGroupFromFile(file os.FileInfo, groupsDir string) (Group, error) {
	// Read the json file and marshall it into a struct
	var group Group
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
func getUserFromFile(file os.FileInfo, usersDir string) (User, error) {
	// Read the json file and marshall it into a struct
	var user User
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
func createAuthorizedKeyFile(user User, sshDir string) error {
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

// Main
func main() {
	del := false

	if len(os.Args) > 1 {
		if os.Args[1] == "-d" {
			del = true
		}
	}

	workDir := getWorkingDirectory()

	err := getMasterFile(workDir)
	check(err)

	err = getUserFile(workDir)
	check(err)

	// map for users to groups
	m := make(map[string][]string)

	// Spin through the groups data bag directory creating groups and building the above map
	// to eventually add the users to the appropriate groups
	groupsDir := workDir + "/data_bags/groups"
	fmt.Printf("Reading directory: %s\n", groupsDir)
	files, _ := ioutil.ReadDir(groupsDir)
	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		group, err := getGroupFromFile(file, groupsDir)
		check(err)

		// Used by group execs
		var cmd *exec.Cmd

		if del {
			fmt.Printf("Deleting group: %v\n", group.ID)
			cmd = exec.Command("groupdel", group.ID)
		} else {
			fmt.Printf("Creating group: %v\n", group.ID)
			cmd = exec.Command("groupadd", group.ID)
		}
		err = cmd.Start()
		check(err)

		err = cmd.Wait()
		check(err)

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
	usersDir := workDir + "/data_bags/users"
	fmt.Printf("Reading directory: %s\n", usersDir)
	files, _ = ioutil.ReadDir(usersDir)
	for _, file := range files {
		if !strings.Contains(file.Name(), ".json") {
			continue
		}

		user, err := getUserFromFile(file, usersDir)
		check(err)

		// Used by user execs
		var cmd *exec.Cmd

		homeDir := "/home/" + user.ID
		if del {
			fmt.Printf("Removing user: %v\n", user.ID)
			cmd = exec.Command("userdel", "--remove", user.ID)
		} else {
			groups := m[user.ID]
			fmt.Printf("Creating user: %v and adding to these groups: %v\n", user.ID, groups)

			if len(groups) == 0 {
				cmd = exec.Command("useradd", "--shell", user.Shell, "--home", homeDir, "--create-home", user.ID)
			} else {
				commaUsers := strings.Join(groups, ",")
				cmd = exec.Command("useradd", "--shell", user.Shell, "--home", homeDir, "--groups", commaUsers, "--create-home", user.ID)
			}
		}

		err = cmd.Start()
		check(err)
		err = cmd.Wait()
		check(err)

		// Create the .ssh directory with only the users accessible permissions then
		// put the ssh key in the directory(which should allow the user to ssh in)
		if !del {
			sshDir := homeDir + "/.ssh"

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
