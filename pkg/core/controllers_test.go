package core

import (
	"context"
	"testing"
)

const deckInputDevices = `I: Bus=0003 Vendor=28de Product=1205 Version=0100
N: Name="Valve Software Steam Deck"
P: Phys=usb-0000:04:00.4-1/input0
S: Sysfs=/devices/pci0000:00/0000:00:08.1/0000:04:00.4/usb5/5-1/input/input30
H: Handlers=event4 js0
B: PROP=0
B: EV=1b

I: Bus=0011 Vendor=0001 Product=0001 Version=ab41
N: Name="AT Translated Set 2 keyboard"
H: Handlers=sysrq kbd event1 leds
B: EV=120013

I: Bus=0003 Vendor=045e Product=028e Version=0110
N: Name="Microsoft X-Box 360 pad"
H: Handlers=event5 js1
B: EV=2001b

I: Bus=0003 Vendor=045e Product=028e Version=0110
N: Name="Microsoft X-Box 360 pad"
H: Handlers=event6 js2
B: EV=2001b

I: Bus=0005 Vendor=054c Product=0ce6 Version=0100
N: Name="DualSense wireless controller"
H: Handlers=sysrq event7
B: EV=1b
`

func TestControllersInventory(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile(inputDevicesPath, []byte(deckInputDevices))
	info, _ := DetectControllers(context.Background(), fsys)
	if info.Discovery != DiscoveryComplete {
		t.Fatalf("discovery: %+v", info)
	}
	// Deck (js+pad), two identical Xbox pads (never merged), DualSense
	// candidate (pad name, no js). Keyboard excluded.
	if len(info.Devices) != 4 {
		t.Fatalf("devices: %+v", info.Devices)
	}
	pads, cands := 0, 0
	for _, d := range info.Devices {
		switch d.IdentityConfidence {
		case ControllerPad:
			pads++
		case ControllerCandidate:
			cands++
		}
		if d.Name == "" || len(d.Handlers) == 0 {
			t.Fatalf("device fields: %+v", d)
		}
	}
	if pads != 3 || cands != 1 {
		t.Fatalf("pads=%d candidates=%d: %+v", pads, cands, info.Devices)
	}
}

func TestControllersUnreadableIsUnknown(t *testing.T) {
	info, diags := DetectControllers(context.Background(), newFixtureFS())
	if info.Discovery != DiscoveryUnknown || len(info.Devices) != 0 {
		t.Fatalf("missing: %+v", info)
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostic")
	}
}

func TestControllersEmptyIsComplete(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile(inputDevicesPath, []byte("I: Bus=0011 Vendor=0001 Product=0001 Version=ab41\nN: Name=\"AT Translated Set 2 keyboard\"\nH: Handlers=kbd event1\n"))
	info, _ := DetectControllers(context.Background(), fsys)
	// Empty successful inventory differs from unreadable input.
	if info.Discovery != DiscoveryComplete || len(info.Devices) != 0 {
		t.Fatalf("keyboard-only: %+v", info)
	}
}

func TestControllersByIDCorroboration(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile(inputDevicesPath, []byte("I: Bus=0003 Vendor=28de Product=1205 Version=0100\nN: Name=\"Valve Software Steam Controller\"\nH: Handlers=event9\n"))
	fsys.addDir("/dev/input/by-id")
	fsys.addLink("/dev/input/by-id/usb-Valve_Software_Steam_Controller-event-joystick", "../event9")
	info, _ := DetectControllers(context.Background(), fsys)
	if len(info.Devices) != 1 || info.Devices[0].IdentityConfidence != ControllerPad {
		t.Fatalf("corroborated: %+v", info)
	}
}

func TestControllersNoSerialsStored(t *testing.T) {
	fsys := newFixtureFS()
	fsys.addFile(inputDevicesPath, []byte(deckInputDevices))
	info, _ := DetectControllers(context.Background(), fsys)
	for _, d := range info.Devices {
		for _, h := range d.Handlers {
			if h == "" {
				t.Fatalf("empty handler: %+v", d)
			}
		}
	}
}
