package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Group -
type Group struct {
	ID     string   `json:"id"`
	Users  []string `json:"users"`
	Admins []string `json:"admins"`
}

// User -
type User struct {
	SSHkeys []string `json:"ssh_keys"`
	ID      string   `json:"id"`
	Shell   string   `json:"shell"`
}

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

func check(e error) {
	if e != nil {
		fmt.Println(e)
		panic(e)
	}
}

// Find a GID by Group Name
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

// Find a UID by User Name
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

func main() {
	del := false

	if len(os.Args) > 1 {
		if os.Args[1] == "-d" {
			del = true
		}
	}

	// get the WORK_DIR environment variable or create a default if oen doesn't exist
	// !!!! need to verify that it exists !!!!
	workDir := os.Getenv("WORK_DIR")
	if workDir == "" {
		fmt.Println("WORK_DIR environment variable not set, using /tmp/eau-work")
		err := os.Mkdir("/tmp/eau-work", 0700)
		check(err)
		workDir = "/tmp/eau-work"
	}

	// map for users to groups
	m := make(map[string][]string)

	// Spin through the groups data bag directory creating groups and building the above map
	// to eventually add the users to the appropriate groups
	groupsDir := workDir + "/data_bags/groups"
	fmt.Printf("Reading directory: %s\n", groupsDir)
	files, _ := ioutil.ReadDir(groupsDir)
	for _, f := range files {
		// Read the json file and marshall it into a struct
		if !strings.Contains(f.Name(), ".json") {
			continue
		}
		groupFile := groupsDir + "/" + f.Name()
		fmt.Printf("Reading file: %s\n", groupFile)
		file, err := ioutil.ReadFile(groupFile)
		check(err)
		var group Group
		json.Unmarshal(file, &group)

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
	// and add the users to the appropriate groups
	usersDir := workDir + "/data_bags/users"
	fmt.Printf("Reading directory: %s\n", usersDir)
	files, _ = ioutil.ReadDir(usersDir)
	for _, f := range files {
		if !strings.Contains(f.Name(), ".json") {
			continue
		}
		// Read the json file and marshall it into a struct
		userFile := usersDir + "/" + f.Name()
		fmt.Printf("Reading file: %s\n", userFile)
		file, err := ioutil.ReadFile(userFile)
		check(err)
		var user User
		json.Unmarshal(file, &user)

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
		}
	}
}
