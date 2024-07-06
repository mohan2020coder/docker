package main
import (
	// Uncomment this block to pass the first stage!
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	// cp "github.com/otiai10/copy"
)
type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	Expires     int    `json:"expires_in"`
	IssuedAt    string `json:"issued_at"`
}
type ManiFest struct {
	Name     string     `json:"name"`
	Tag      string     `json:"tag"`
	FSLayers []fsLayers `json:"fsLayers"`
}
type fsLayers struct {
	BlobSum string `json:"blobSum"`
}
// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	img := os.Args[2]
	split := strings.Split(img, ":")
	repo := "library"
	image := split[0]
	tag := "latest"
	if len(split) == 2 {
		tag = split[1]
	}
	request, err := http.NewRequest("GET", fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo+"/"+image), nil)
	if err != nil {
		fmt.Printf("ERR!! %+v", err)
	}
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(request)
	var result TokenResponse
	json.NewDecoder(resp.Body).Decode(&result)
	// fmt.Printf("\n\nTOKEN => %+v\n\n", result.Token)
	manifestReq, err := http.NewRequest("GET", fmt.Sprintf("https://registry.hub.docker.com/v2/%s/manifests/%s", repo+"/"+image, tag), nil)
	if err != nil {
		fmt.Printf("ERR!! %+v", err)
	}
	manifestReq.Header.Add("Authorization", "Bearer "+strings.TrimSpace(result.Token))
	manifestReq.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v1+json")
	mani, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		fmt.Printf("ERRRR => %+v", err)
	}
	var manifest ManiFest
	json.NewDecoder(mani.Body).Decode(&manifest)
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID,
		// Cloneflags: syscall.CLONE_NEWPID,
	}
	if err := os.MkdirAll("tmp_dir/dev/null", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("tmp_dir/usr/bin/", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("tmp_dir/bin/", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll("tmp_dir/")
	input, err := ioutil.ReadFile(command)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll("tmp_dir/")
	for _, value := range manifest.FSLayers {
		req, err := http.NewRequest("GET", "https://registry-1.docker.io/v2/library/"+image+"/blobs/"+value.BlobSum, nil)
		if err != nil {
			fmt.Println("er1")
		}
		req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(result.Token))
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("er2")
		}
		defer resp.Body.Close()
		f, e := os.Create("tmp_dir/output")
		if e != nil {
			panic(e)
		}
		defer f.Close()
		f.ReadFrom(resp.Body)
		_, err = exec.Command("tar", "xf", "tmp_dir/output", "-C", "tmp_dir/").Output()
		if err != nil {
			fmt.Printf("OUT ERR untar => %+v", err)
		}
		// fmt.Printf("output => %+v", out)
		os.RemoveAll("tmp_dir/output")
	}
	if err := syscall.Chroot("tmp_dir/"); err != nil {
		log.Fatal(err)