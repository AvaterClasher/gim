package main

import (
	"io"
	"log"
	"os"
	"syscall"
	"unsafe"
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
	origTermios *Termios
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
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS+1), uintptr(unsafe.Pointer(termios))); err != 0 {
		return err
	}
	return nil
}

func TcGetAttr(fd uintptr) *Termios {
	var termios = &Termios{}
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(termios))); err != 0 {
		log.Fatalf("Problem getting termial attributes: %s\n", err)
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

func editorReadKey() byte {
	var buffer [1]byte
	var cc int
	var err error
	for cc, err = os.Stdin.Read(buffer[:]); cc != 1; cc, err = os.Stdin.Read(buffer[:]) {
		// Blank
	}
	if err != nil {
		die(err)
	}
	return buffer[0]

}

/* Input */

func editorProcessKeypress() {
	c := editorReadKey()
	switch c {
	case ('q' & 0x1f):
		io.WriteString(os.Stdout, "\x1b[2J")
		io.WriteString(os.Stdout, "\x1b[H")
		disableRawMode()
		os.Exit(0)
	}
}

/* Output */

func editorRefreshScreen() {
	io.WriteString(os.Stdout, "\x1b[2J")
	io.WriteString(os.Stdout, "\x1b[H")
	editorDrawRows()
	io.WriteString(os.Stdout, "\x1b[H")
}

func editorDrawRows() {
	for y := 0; y < 24; y++ {
		io.WriteString(os.Stdout, "~\r\n")
	}
}

/* INIT / MAIN FUNC */

func main() {
	enableRawMode()
	defer disableRawMode()
	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
