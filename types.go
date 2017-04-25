package main

// ArgoGroup -
type ArgoGroup struct {
	ID     string   `json:"id"`
	Users  []string `json:"users"`
	Admins []string `json:"admins"`
}

// ArgoUser -
type ArgoUser struct {
	SSHkeys []string `json:"ssh_keys"`
	ID      string   `json:"id"`
	Shell   string   `json:"shell"`
}
