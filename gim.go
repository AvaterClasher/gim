package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"syscall"
	"unsafe"
)

/* Defines */

const GIM_VERSION = "0.0.1"
const (
	ARROW_LEFT  = 1000 + iota
	ARROW_RIGHT = 1000 + iota
	ARROW_UP    = 1000 + iota
	ARROW_DOWN  = 1000 + iota
)

/* DATA */

type Termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [20]byte
	Ispeed uint32
	Ospeed uint32
}

type editorConfig struct {
	cy          int
	cx          int
	screenRows  int
	screenCols  int
	origTermios *Termios
}

type WinSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

var E editorConfig

/* TERMINAL */

func die(err error) {
	disableRawMode()
	io.WriteString(os.Stdout, "\x1b[2J")
	io.WriteString(os.Stdout, "\x1b[H")
	log.Fatal(err)
}

func TcSetAttr(fd uintptr, termios *Termios) error {
	// TCSETS+1 == TCSETSW, because TCSAFLUSH doesn't exist
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS+1), uintptr(unsafe.Pointer(termios))); err != 0 {
		return err
	}
	return nil
}

func TcGetAttr(fd uintptr) *Termios {
	var termios = &Termios{}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(termios))); err != 0 {
		log.Fatalf("Problem getting terminal attributes: %s\n", err)
	}
	return termios
}

func enableRawMode() {
	E.origTermios = TcGetAttr(os.Stdin.Fd())
	var raw Termios
	raw = *E.origTermios
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.BRKINT
	raw.Oflag &^= syscall.OPOST
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN+1] = 0
	raw.Cc[syscall.VTIME+1] = 1
	if e := TcSetAttr(os.Stdin.Fd(), &raw); e != nil {
		log.Fatalf("Problem enabling raw mode: %s\n", e)
	}
}

func disableRawMode() {
	if e := TcSetAttr(os.Stdin.Fd(), E.origTermios); e != nil {
		log.Fatalf("Problem disabling raw mode: %s\n", e)
	}
}

func editorReadKey() int {
	var buffer [1]byte
	var cc int
	var err error
	for cc, err = os.Stdin.Read(buffer[:]); cc != 1; cc, err = os.Stdin.Read(buffer[:]) {
		// Blank
	}
	if err != nil {
		die(err)
	}
	if buffer[0] == '\x1b' {
		var seq [2]byte
		cc, _ = os.Stdin.Read(seq[:])
		if cc != 2 {
			return '\x1b'
		}

		if seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return ARROW_UP
			case 'B':
				return ARROW_DOWN
			case 'C':
				return ARROW_RIGHT
			case 'D':
				return ARROW_LEFT
			}
		}

		return '\x1b'
	}
	return int(buffer[0])

}

func getCursorPosition(rows *int, cols *int) int {
	io.WriteString(os.Stdout, "\x1b[6n")
	var buf []byte
	var buffer [1]byte
	var cc int
	for cc, _ = os.Stdin.Read(buffer[:]); cc == 1; cc, _ = os.Stdin.Read(buffer[:]) {
		if buffer[0] == 'R' {
			break
		}
		buf = append(buf, buffer[0])
	}
	if string(buf[0:2]) != "\x1b[" {
		log.Printf("Failed to read rows;cols from tty\n")
		return -1
	}
	if n, e := fmt.Sscanf(string(buf[2:]), "%d;%d", rows, cols); n != 2 || e != nil {
		if e != nil {
			log.Printf("getCursorPosition: fmt.Sscanf() failed: %s\n", e)
		}
		if n != 2 {
			log.Printf("getCursorPosition: got %d items, wanted 2\n", n)
		}
		return -1
	}
	return 0
}

func getWindowSize(rows *int, cols *int) int {
	var w WinSize
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&w)),
	)
	if err != 0 { // type syscall.Errno
		io.WriteString(os.Stdout, "\x1b[999C\x1b[999B")
		return getCursorPosition(rows, cols)
	} else {
		*rows = int(w.Row)
		*cols = int(w.Col)
		return 0
	}
	return -1
}

/* Input */

func editorMoveCursor(key int) {
	switch key {
	case ARROW_LEFT:
		E.cx--
	case ARROW_RIGHT:
		E.cx++
	case ARROW_UP:
		E.cy--
	case ARROW_DOWN:
		E.cy++
	}
}

func editorProcessKeypress() {
	c := editorReadKey()
	switch c {
	case ('q' & 0x1f):
		io.WriteString(os.Stdout, "\x1b[2J")
		io.WriteString(os.Stdout, "\x1b[H")
		disableRawMode()
		os.Exit(0)
	case ARROW_LEFT, ARROW_RIGHT, ARROW_UP, ARROW_DOWN:
		editorMoveCursor(c)
	}
}

/* Append Buffer */

type abuf struct {
	buf []byte
}

func (p abuf) String() string {
	return string(p.buf)
}

func (p *abuf) abAppend(s string) {
	p.buf = append(p.buf, []byte(s)...)
}

/* Output */

func editorRefreshScreen() {
	var ab abuf
	ab.abAppend("\x1b[25l")
	ab.abAppend("\x1b[H")
	editorDrawRows(&ab)
	ab.abAppend(fmt.Sprintf("\x1b[%d;%dH", E.cy+1, E.cx+1))
	ab.abAppend("\x1b[25h")
	io.WriteString(os.Stdout, ab.String())
}

func editorDrawRows(ab *abuf) {
	for y := 0; y < E.screenRows-1; y++ {
		if y == E.screenRows/3 {
			w := fmt.Sprintf("Gim editor -- version %s", GIM_VERSION)
			if len(w) > E.screenCols {
				w = w[0:E.screenCols]
			}
			pad := "~ "
			for padding := (E.screenCols - len(w)) / 2; padding > 0; padding-- {
				ab.abAppend(pad)
				pad = " "
			}
			ab.abAppend(w)
		} else {
			ab.abAppend("~")
		}
		ab.abAppend("\x1b[K")
		if y < E.screenRows-1 {
			ab.abAppend("\r\n")
		}
	}
}

/* INIT / MAIN FUNC */

func initEditor() {
	if getWindowSize(&E.screenRows, &E.screenCols) == -1 {
		die(fmt.Errorf("couldn't get screen size"))
	}
}

func main() {
	enableRawMode()
	defer disableRawMode()
	initEditor()
	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
