package sshagent

func SetAgentAddress(fn func() string) {
	sockNameFunc = fn
}
