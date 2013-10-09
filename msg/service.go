package msg

type Service struct {
	UUID        string
	Name        string
	Version     string
	Environment string
	Region      string
	Host        string
	Port        uint16
	TTL         uint32 // Seconds
}
