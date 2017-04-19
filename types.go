package main

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
