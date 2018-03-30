package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

func main() {
	systray.Run(onReady, nil)
}

func onReady() {
	systray.SetTitle("Loading...")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	go func() {
		for {
			proc := getCurrentProcesses()
			systray.MenuItems = proc
			numProc := strconv.Itoa(len(proc))
			systray.SetTitle("Utilized Ports: " + numProc)
			mQuit = systray.AddMenuItem("Quit", "Quit the whole app")
			time.Sleep(15 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func getCurrentProcesses() map[int32]*systray.MenuItem {
	var b bytes.Buffer
	var pid string

	// Run `lsof -i | grep LISTEN` to get listening ports
	c1 := exec.Command("lsof", "-i")
	c2 := exec.Command("grep", "LISTEN")

	r, w := io.Pipe()
	c1.Stdout = w
	c2.Stdin = r
	c2.Stdout = &b

	c1.Start()
	c2.Start()
	c1.Wait()
	w.Close()
	c2.Wait()

	s := b.String()
	// The returned output from c2 is new line separated items
	lines := strings.Split(s, "\n")

	// Hide anything that is currently in the systray.MenuItems
	for _, item := range systray.MenuItems {
		item.Hide()
	}

	m := make(map[int32]*systray.MenuItem)

	for idx, allProc := range lines {
		// Only take IPv4 ports for now
		if strings.Contains(allProc, "IPv4") {
			// TODO Could these be tab characters?
			listProc := strings.Split(allProc, " ")
			// Hacky way to get the process ID
			for _, item := range listProc {
				if _, err := strconv.Atoi(item); err == nil {
					pid = item
				}
				// Hacky way to get the item with the port
				if strings.ContainsAny(item, ":") {
					port := strings.Split(item, ":")[1]
					ID := int32(idx)
					Title := port + " -- " + listProc[0]
					Tooltip := "PID: " + pid
					Disabled := false
					Checked := false
					a := &systray.MenuItem{ClickedCh: nil, ID: ID, Title: Title,
						Tooltip: Tooltip, Disabled: Disabled, Checked: Checked}
					m[int32(idx)] = a
					a.Update()
					// We hid things before so we need to show them now
					a.Show()
				}
			}
		}
	}
	return m
}

func getIcon(s string) []byte {
	b, err := ioutil.ReadFile(s)
	if err != nil {
		fmt.Print(err)
	}
	return b
}
