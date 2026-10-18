//go:build darwin

package goclip

// The macOS pasteboard, spoken to directly through the Objective-C runtime.
//
// pbcopy and pbpaste used to be the only way in, and they cannot be trusted
// with text: they convert between the pasteboard and their standard streams
// in the encoding of the process locale. A process without a UTF-8 locale --
// one started from the Dock, or from a non-login shell such as the one
// far2l's terminal runs -- got every non-ASCII character back as '?' on paste
// and stored MacRoman mojibake on copy (unxed/f4#2057). The pasteboard itself
// holds Unicode, and asking it for public.utf8-plain-text needs no locale.
//
// The runtime is reached through purego, so the package still builds with
// CGO_ENABLED=0. AppKit is loaded on first use, not at startup.

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	appKitPath           = "/System/Library/Frameworks/AppKit.framework/AppKit"
	nsUTF8StringEncoding = 4
	// The value of NSPasteboardTypeString.
	utf8PlainTextType = "public.utf8-plain-text"
)

var errPasteboardRefused = errors.New("goclip: the pasteboard refused the text")

type nativePasteboard struct {
	pasteboardClass, stringClass, poolClass objc.Class

	alloc, init, release             objc.SEL
	generalPasteboard, clearContents objc.SEL
	stringForType, setStringForType  objc.SEL
	dataUsingEncoding, bytes, length objc.SEL
	initWithBytesLengthEncoding      objc.SEL

	// The type string, owned for the life of the process.
	stringType objc.ID
}

var (
	nativeOnce sync.Once
	native     *nativePasteboard
	nativeErr  error
)

// loadNativePasteboard opens AppKit once; a failure is remembered, and the
// driver stays on pbcopy and pbpaste from then on.
func loadNativePasteboard() (*nativePasteboard, error) {
	nativeOnce.Do(func() { native, nativeErr = openNativePasteboard() })
	return native, nativeErr
}

func openNativePasteboard() (*nativePasteboard, error) {
	if _, err := purego.Dlopen(appKitPath, purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
		return nil, err
	}
	p := &nativePasteboard{
		pasteboardClass: objc.GetClass("NSPasteboard"),
		stringClass:     objc.GetClass("NSString"),
		poolClass:       objc.GetClass("NSAutoreleasePool"),

		alloc:                       objc.RegisterName("alloc"),
		init:                        objc.RegisterName("init"),
		release:                     objc.RegisterName("release"),
		generalPasteboard:           objc.RegisterName("generalPasteboard"),
		clearContents:               objc.RegisterName("clearContents"),
		stringForType:               objc.RegisterName("stringForType:"),
		setStringForType:            objc.RegisterName("setString:forType:"),
		dataUsingEncoding:           objc.RegisterName("dataUsingEncoding:"),
		bytes:                       objc.RegisterName("bytes"),
		length:                      objc.RegisterName("length"),
		initWithBytesLengthEncoding: objc.RegisterName("initWithBytes:length:encoding:"),
	}
	if p.pasteboardClass == 0 || p.stringClass == 0 || p.poolClass == 0 {
		return nil, errors.New("goclip: AppKit loaded without NSPasteboard")
	}
	if p.stringType = p.newString(utf8PlainTextType); p.stringType == 0 {
		return nil, errors.New("goclip: could not make the pasteboard type string")
	}
	return p, nil
}

// withPool runs fn inside an autorelease pool. A goroutine has no pool of its
// own, so without one every autoreleased object a call returns would leak;
// the pool belongs to the thread, so the goroutine stays on it throughout.
func (p *nativePasteboard) withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(p.poolClass).Send(p.alloc).Send(p.init)
	defer pool.Send(p.release)
	fn()
}

// newString makes an NSString the caller releases. The bytes go in with
// their length, so a NUL inside the text survives, which a C string would cut.
func (p *nativePasteboard) newString(s string) objc.ID {
	buf := make([]byte, len(s)+1) // never empty: no NULL pointer for ""
	copy(buf, s)
	str := objc.ID(p.stringClass).Send(p.alloc).Send(p.initWithBytesLengthEncoding,
		unsafe.Pointer(&buf[0]), uint(len(s)), uint(nsUTF8StringEncoding))
	runtime.KeepAlive(buf)
	return str
}

func (p *nativePasteboard) readText() (text string, err error) {
	p.withPool(func() {
		pb := objc.ID(p.pasteboardClass).Send(p.generalPasteboard)
		if pb == 0 {
			err = ErrUnavailable
			return
		}
		str := pb.Send(p.stringForType, p.stringType)
		if str == 0 {
			err = ErrEmpty
			return
		}
		data := str.Send(p.dataUsingEncoding, uint(nsUTF8StringEncoding))
		if data == 0 {
			err = ErrUnsupportedFormat
			return
		}
		n := objc.Send[uint](data, p.length)
		if n == 0 {
			err = ErrEmpty
			return
		}
		ptr := objc.Send[unsafe.Pointer](data, p.bytes)
		text = string(unsafe.Slice((*byte)(ptr), n))
	})
	return text, err
}

func (p *nativePasteboard) writeText(text string) (err error) {
	// initWithBytes:length:encoding: answers nil for bytes that are not
	// UTF-8, and nil would put nothing on the pasteboard.
	text = strings.ToValidUTF8(text, "�")
	p.withPool(func() {
		pb := objc.ID(p.pasteboardClass).Send(p.generalPasteboard)
		if pb == 0 {
			err = ErrUnavailable
			return
		}
		str := p.newString(text)
		if str == 0 {
			err = ErrUnsupportedFormat
			return
		}
		defer str.Send(p.release)
		pb.Send(p.clearContents)
		if !objc.Send[bool](pb, p.setStringForType, str, p.stringType) {
			err = errPasteboardRefused
		}
	})
	return err
}

func (p *nativePasteboard) clear() (err error) {
	p.withPool(func() {
		pb := objc.ID(p.pasteboardClass).Send(p.generalPasteboard)
		if pb == 0 {
			err = ErrUnavailable
			return
		}
		pb.Send(p.clearContents)
	})
	return err
}
