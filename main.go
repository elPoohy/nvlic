package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"bufio"
	"strings"

	"golang.org/x/crypto/ssh"

	"io/ioutil"
	"net/http"

	"html/template"
)

// Config профиль конфига
type Config struct {
	Hosts    []string
	Profiles []Profiles
	Api      string
}

// Profiles сами понимаете
type Profiles struct {
	Profile string
	Mem     int
}

// Licenses ...
type Licenses struct {
	ID, Aval, Total int
	Name, Exp       string
}

// Hosts ...
type Hosts struct {
	Used, Aval, Total       int
	ID, Name, Card, Profile string
}

// HProfiles ...
type HProfiles struct {
	Used, Aval, Total int
	Name, Profile     string
}

// HTMLOut What's this
type HTMLOut struct {
	Title     string
	Licenses  []Licenses
	Hosts     []Hosts
	HProfiles []HProfiles
	AProfiles []HProfiles
	Time      string
}

// ClearAProfiles clear
func (HTMLOut *HTMLOut) CleatAProfiles() []HProfiles {
	HTMLOut.AProfiles = nil
	return HTMLOut.AProfiles
}

// AddAProfiles add
func (HTMLOut *HTMLOut) AddAProfiles(item HProfiles) []HProfiles {
	finded := false
	var idx int
	for i := range HTMLOut.AProfiles {
		if HTMLOut.AProfiles[i].Profile == item.Profile {
			idx = i
			finded = true
		}
	}
	if finded {
		HTMLOut.AProfiles[idx].Used += item.Used
		HTMLOut.AProfiles[idx].Aval += item.Aval
		HTMLOut.AProfiles[idx].Total += item.Total
	} else {
		HTMLOut.AProfiles = append(HTMLOut.AProfiles, item)
	}
	return HTMLOut.AProfiles
}

// ClearLicense clear
func (HTMLOut *HTMLOut) ClearLicense() []Licenses {
	HTMLOut.Licenses = nil
	return HTMLOut.Licenses
}

// AddLicense clear
func (HTMLOut *HTMLOut) AddLicense(item Licenses) []Licenses {
	HTMLOut.Licenses = append(HTMLOut.Licenses, item)
	return HTMLOut.Licenses
}

// ClearHosts clear
func (HTMLOut *HTMLOut) ClearHosts() []Hosts {
	HTMLOut.Hosts = nil
	return HTMLOut.Hosts
}

// AddHosts clear
func (HTMLOut *HTMLOut) AddHosts(item Hosts) []Hosts {
	HTMLOut.Hosts = append(HTMLOut.Hosts, item)
	return HTMLOut.Hosts
}

// ClearHProfiles clear
func (HTMLOut *HTMLOut) ClearHProfiles() []HProfiles {
	HTMLOut.HProfiles = nil
	return HTMLOut.HProfiles
}

// AddHProfiles clear
func (HTMLOut *HTMLOut) AddHProfiles(item HProfiles) []HProfiles {
	HTMLOut.HProfiles = append(HTMLOut.HProfiles, item)
	return HTMLOut.HProfiles
}

// NvidiaLics все лиццензии
type NvidiaLics []struct {
	ID             int    `json:"id"`
	Type           string `json:"type"`
	FeatureName    string `json:"featureName"`
	FeatureVersion string `json:"featureVersion"`
	Expiry         string `json:"expiry"`
	FeatureCount   int    `json:"featureCount"`
	OverdraftCount int    `json:"overdraftCount"`
	Used           int    `json:"used"`
	FeatureID      string `json:"featureId"`
	Concurrent     bool   `json:"concurrent"`
	Uncounted      bool   `json:"uncounted"`
	Reserved       int    `json:"reserved"`
}

// vProfiles sdsd
type vProfiles struct {
	Used, Total, Aval int
}

// connectToHost подключение по SSH и выполнение заданной комманды
func connectToHost(user, host, pass string) (*ssh.Client, *ssh.Session, error) {

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}

// GetAllData it's all I can get
func (data *HTMLOut) GetAllData(config Config) {
	HostsTotal := make(map[string]map[string]int)
	HostsAval := make(map[string]map[string]int)
	HostsUsed := make(map[string]map[string]int)
	data.ClearLicense()
	response, err := http.Get(fmt.Sprintf("%s%s", config.Api, "features"))
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	} else {
		responseData, _ := ioutil.ReadAll(response.Body)
		var NvidiaLics NvidiaLics
		json.Unmarshal(responseData, &NvidiaLics)
		for i := 0; i < len(NvidiaLics); i++ {
			data.AddLicense(Licenses{
				ID:    NvidiaLics[i].ID,
				Name:  NvidiaLics[i].FeatureName,
				Exp:   NvidiaLics[i].Expiry,
				Aval:  NvidiaLics[i].FeatureCount - NvidiaLics[i].Used,
				Total: NvidiaLics[i].FeatureCount,
			})
		}
	}
	data.ClearHosts()
	for _, host := range config.Hosts {
		gpuname := ""
		gpuid := ""
		gpuprofile := ""
		gpuprofilecount := 0
		gpuprofilemax := 0
		// gpuutil := ""
		client, session, err := connectToHost("root", fmt.Sprintf(`%s:22`, host), "@WSX3edc$RFV5tgb")
		if err != nil {
			fmt.Printf(`Connection to host %s timed out.`, host)
		} else {
			out, err := session.CombinedOutput("nvidia-smi vgpu")
			if err != nil {
				panic(err)
			}
			client.Close()
			outStrings := string(out)
			//fmt.Println(outStrings)
			scanner := bufio.NewScanner(strings.NewReader(outStrings))
			for scanner.Scan() {
				s := scanner.Text()
				findGPU := regexp.MustCompile(`^\|   (\d)  ([\d\w ]+) +\|.*$`)
				findGRID := regexp.MustCompile(`.*GRID.*`)
				if findGPU.MatchString(s) {
					if gpuname != "" {
						data.AddHosts(Hosts{
							ID:      gpuid,
							Name:    host,
							Card:    gpuname,
							Profile: gpuprofile,
							Used:    gpuprofilecount,
							Aval:    gpuprofilemax - gpuprofilecount,
							Total:   gpuprofilemax,
						})
					}
					gpuname = strings.Replace(s[7:33], "  ", "", -1)
					gpuid = s[4:5]
					gpuprofile = "—"
					gpuprofilecount = 0
					gpuprofilemax = 0
				} else if findGRID.MatchString(s) {
					if gpuprofilecount == 0 {
						gpuprofile = strings.Replace(s[24:33], " ", "", -1)
						totalMem := 0
						for _, v := range config.Profiles {
							if v.Profile == gpuprofile[0:strings.Index(gpuprofile, "-")] {
								totalMem = v.Mem
							}
						}
						if totalMem == 0 {
							fmt.Printf("!WARNING! %s is unknown profile type. Please update your conf.json", gpuprofile)
							fmt.Println()
						}
						i, _ := strconv.ParseInt(gpuprofile[strings.Index(gpuprofile, "-")+1:len(gpuprofile)-1], 10, 64)
						gpuprofilemax = totalMem / int(i)
						if _, ok := HostsTotal[host]; !ok {
							HostsTotal[host] = make(map[string]int)
							HostsAval[host] = make(map[string]int)
							HostsUsed[host] = make(map[string]int)
						}
						if _, ok := HostsTotal[host][gpuprofile]; !ok {
							HostsTotal[host][gpuprofile] = gpuprofilemax
							HostsUsed[host][gpuprofile] = 0
							HostsAval[host][gpuprofile] = gpuprofilemax
						} else {
							HostsTotal[host][gpuprofile] += gpuprofilemax
							HostsAval[host][gpuprofile] += gpuprofilemax
						}
					}
					gpuprofilecount++
					HostsUsed[host][gpuprofile]++
					HostsAval[host][gpuprofile]--
				}
			}
			data.AddHosts(Hosts{
				ID:      gpuid,
				Name:    host,
				Card:    gpuname,
				Profile: gpuprofile,
				Used:    gpuprofilecount,
				Aval:    gpuprofilemax - gpuprofilecount,
				Total:   gpuprofilemax,
			})
		}
	}
	data.ClearHProfiles()
	data.CleatAProfiles()
	for host := range HostsTotal {
		for gpuprofile := range HostsTotal[host] {
			data.AddHProfiles(HProfiles{
				Name:    host,
				Profile: gpuprofile,
				Total:   HostsTotal[host][gpuprofile],
				Aval:    HostsAval[host][gpuprofile],
				Used:    HostsUsed[host][gpuprofile],
			})
			data.AddAProfiles(HProfiles{
				Name:    "",
				Profile: gpuprofile,
				Total:   HostsTotal[host][gpuprofile],
				Aval:    HostsAval[host][gpuprofile],
				Used:    HostsUsed[host][gpuprofile],
			})
		}
	}
}

func main() {
	tmpl, _ := template.ParseFiles("index.html")
	data := new(HTMLOut)
	data.Title = "Статус лицензий"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, data)
	})
	fmt.Println("Server is listening...")
	go http.ListenAndServe(":8181", nil)
	for {
		file, _ := os.Open("conf.json")
		decoder := json.NewDecoder(file)
		config := new(Config)
		err := decoder.Decode(&config)
		nextdata := new(HTMLOut)
		if err != nil {
			fmt.Println("Ошибка файла конфигурации")
		}
		nextdata.GetAllData(*config)
		nextdata.Time = time.Now().Format("2006-01-02 15:04:05")
		nextdata.Title = fmt.Sprintf(`Статус лицензий [%s]`, time.Now().Format("15:04"))
		data = nextdata
		nextdata = nil
		time.Sleep(5 * time.Minute)
	}
}
