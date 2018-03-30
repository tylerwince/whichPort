/*
Package systray is a cross platfrom Go library to place an icon and menu in the
notification area.
Supports Windows, Mac OSX and Linux currently.
Methods can be called from any goroutine except Run(), which should be called
at the very beginning of main() to lock at main thread.
*/
package systray

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/getlantern/golog"
)

var (
	hasStarted = int64(0)
	hasQuit    = int64(0)
	MenuItems  = map[int32]*MenuItem{}
)

// MenuItem is used to keep track each menu item of systray
// Don't create it directly, use the one systray.AddMenuItem() returned
type MenuItem struct {
	// ClickedCh is the channel which will be notified when the menu item is clicked
	ClickedCh chan struct{}

	// ID uniquely identify a menu item, not supposed to be modified
	ID int32
	// Title is the text shown on menu item
	Title string
	// Tooltip is the text shown when pointing to menu item
	Tooltip string
	// Disabled menu item is grayed out and has no effect when clicked
	Disabled bool
	// Checked menu item has a tick before the title
	Checked bool
}

var (
	log = golog.LoggerFor("systray")

	systrayReady  func()
	systrayExit   func()
	menuItems     = make(map[int32]*MenuItem)
	menuItemsLock sync.RWMutex

	currentID = int32(-1)
)

// Run initializes GUI and starts the event loop, then invokes the onReady
// callback.
// It blocks until systray.Quit() is called.
// Should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	runtime.LockOSThread()
	atomic.StoreInt64(&hasStarted, 1)

	if onReady == nil {
		systrayReady = func() {}
	} else {
		// Run onReady on separate goroutine to avoid blocking event loop
		readyCh := make(chan interface{})
		go func() {
			<-readyCh
			onReady()
		}()
		systrayReady = func() {
			close(readyCh)
		}
	}

	// unlike onReady, onExit runs in the event loop to make sure it has time to
	// finish before the process terminates
	if onExit == nil {
		onExit = func() {}
	}
	systrayExit = onExit

	nativeLoop()
}

// Quit the systray
func Quit() {
	if atomic.LoadInt64(&hasStarted) == 1 && atomic.CompareAndSwapInt64(&hasQuit, 0, 1) {
		quit()
	}
}

// AddMenuItem adds menu item with designated title and tooltip, returning a channel
// that notifies whenever that menu item is clicked.
//
// It can be safely invoked from different goroutines.
func AddMenuItem(title string, tooltip string) *MenuItem {
	ID := atomic.AddInt32(&currentID, 1)
	item := &MenuItem{nil, ID, title, tooltip, false, false}
	item.ClickedCh = make(chan struct{})
	item.Update()
	return item
}

// AddSeparator adds a separator bar to the menu
func AddSeparator() {
	addSeparator(atomic.AddInt32(&currentID, 1))
}

// SetTitle set the text to display on a menu item
func (item *MenuItem) SetTitle(title string) {
	item.Title = title
	item.Update()
}

// SetTooltip set the tooltip to show when mouse hover
func (item *MenuItem) SetTooltip(tooltip string) {
	item.Tooltip = tooltip
	item.Update()
}

// Disabled checkes if the menu item is disabled
func (item *MenuItem) IsDisabled() bool {
	return item.Disabled
}

// Enable a menu item regardless if it's previously enabled or not
func (item *MenuItem) Enable() {
	item.Disabled = false
	item.Update()
}

// Disable a menu item regardless if it's previously disabled or not
func (item *MenuItem) Disable() {
	item.Disabled = true
	item.Update()
}

// Hide hides a menu item
func (item *MenuItem) Hide() {
	hideMenuItem(item)
}

// Show shows a previously hidden menu item
func (item *MenuItem) Show() {
	showMenuItem(item)
}

// Checked returns if the menu item has a check mark
func (item *MenuItem) IsChecked() bool {
	return item.Checked
}

// Check a menu item regardless if it's previously checked or not
func (item *MenuItem) Check() {
	item.Checked = true
	item.Update()
}

// Uncheck a menu item regardless if it's previously unchecked or not
func (item *MenuItem) Uncheck() {
	item.Checked = false
	item.Update()
}

// Update propogates changes on a menu item to systray
func (item *MenuItem) Update() {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()
	menuItems[item.ID] = item
	addOrUpdateMenuItem(item)
}

func systrayMenuItemSelected(ID int32) {
	menuItemsLock.RLock()
	item := menuItems[ID]
	menuItemsLock.RUnlock()
	select {
	case item.ClickedCh <- struct{}{}:
	// in case no one waiting for the channel
	default:
	}
}
