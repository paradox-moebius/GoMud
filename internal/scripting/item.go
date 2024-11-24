package scripting

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/dop251/goja"
	"github.com/volte6/gomud/internal/items"
)

var (
	itemVMCache       = make(map[string]*VMWrapper)
	scriptItemTimeout = 10 * time.Millisecond
)

func PruneItemVMs(instanceIds ...int) {

}

func TryItemScriptEvent(eventName string, item items.Item, userId int) (bool, error) {

	sItem := GetItem(item)

	timestart := time.Now()
	defer func() {
		slog.Debug("TryItemScriptEvent()", "eventName", eventName, "item", item, "time", time.Since(timestart))
	}()

	vmw, err := getItemVM(sItem)
	if err != nil {
		return false, err
	}

	if onCommandFunc, ok := vmw.GetFunction(eventName); ok {

		sUser := GetActor(userId, 0)
		sRoom := GetRoom(sUser.GetRoomId())

		tmr := time.AfterFunc(scriptItemTimeout, func() {
			vmw.VM.Interrupt(errTimeout)
		})
		res, err := onCommandFunc(goja.Undefined(),
			vmw.VM.ToValue(sUser),
			vmw.VM.ToValue(sItem),
			vmw.VM.ToValue(sRoom),
		)
		vmw.VM.ClearInterrupt()
		tmr.Stop()

		if err != nil {

			// Wrap the error
			finalErr := fmt.Errorf("%s(): %w", eventName, err)

			if _, ok := finalErr.(*goja.Exception); ok {
				slog.Error("JSVM", "exception", finalErr)
				return false, finalErr
			} else if errors.Is(finalErr, errTimeout) {
				slog.Error("JSVM", "interrupted", finalErr)
				return false, finalErr
			}

			slog.Error("JSVM", "error", finalErr)
			return false, finalErr
		}

		if eventName != `onLost` {
			// Save any changed that might have happened to the item
			sUser.characterRecord.UpdateItem(item, *sItem.itemRecord)
		}

		if boolVal, ok := res.Export().(bool); ok {
			return boolVal, nil
		}

	}

	return false, nil
}

func TryItemCommand(cmd string, item items.Item, userId int) (bool, error) {

	sItem := GetItem(item)

	timestart := time.Now()
	defer func() {
		slog.Debug("TryItemCommand()", "cmd", cmd, "itemId", item.ItemId, "userId", userId, "time", time.Since(timestart))
	}()

	vmw, err := getItemVM(sItem)
	if err != nil {
		return false, err
	}

	if onCommandFunc, ok := vmw.GetFunction(`onCommand_` + cmd); ok {

		sUser := GetActor(userId, 0)
		sRoom := GetRoom(sUser.GetRoomId())

		tmr := time.AfterFunc(scriptItemTimeout, func() {
			vmw.VM.Interrupt(errTimeout)
		})
		res, err := onCommandFunc(goja.Undefined(),
			vmw.VM.ToValue(sUser),
			vmw.VM.ToValue(sItem),
			vmw.VM.ToValue(sRoom),
		)
		vmw.VM.ClearInterrupt()
		tmr.Stop()

		if err != nil {

			// Wrap the error
			finalErr := fmt.Errorf("onCommand_%s(): %w", cmd, err)

			if _, ok := finalErr.(*goja.Exception); ok {
				slog.Error("JSVM", "exception", finalErr)
				return false, finalErr
			} else if errors.Is(finalErr, errTimeout) {
				slog.Error("JSVM", "interrupted", finalErr)
				return false, finalErr
			}

			slog.Error("JSVM", "error", finalErr)
			return false, finalErr
		}

		// Save any changed that might have happened to the item
		sUser.characterRecord.UpdateItem(item, *sItem.itemRecord)

		if boolVal, ok := res.Export().(bool); ok {
			return boolVal, nil
		}

	} else if onCommandFunc, ok := vmw.GetFunction(`onCommand`); ok {

		sUser := GetActor(userId, 0)
		sRoom := GetRoom(sUser.GetRoomId())

		tmr := time.AfterFunc(scriptItemTimeout, func() {
			vmw.VM.Interrupt(errTimeout)
		})
		res, err := onCommandFunc(goja.Undefined(),
			vmw.VM.ToValue(cmd),
			vmw.VM.ToValue(sUser),
			vmw.VM.ToValue(sItem),
			vmw.VM.ToValue(sRoom),
		)
		vmw.VM.ClearInterrupt()
		tmr.Stop()

		if err != nil {

			// Wrap the error
			finalErr := fmt.Errorf("onCommand(): %w", err)

			if _, ok := finalErr.(*goja.Exception); ok {
				slog.Error("JSVM", "exception", finalErr)
				return false, finalErr
			} else if errors.Is(finalErr, errTimeout) {
				slog.Error("JSVM", "interrupted", finalErr)
				return false, finalErr
			}

			slog.Error("JSVM", "error", finalErr)
			return false, finalErr
		}

		// Save any changed that might have happened to the item
		sUser.characterRecord.UpdateItem(item, *sItem.itemRecord)

		if boolVal, ok := res.Export().(bool); ok {
			return boolVal, nil
		}

	}

	return false, nil
}

func getItemVM(sItem *ScriptItem) (*VMWrapper, error) {

	scriptId := strconv.Itoa(sItem.ItemId())

	if vm, ok := itemVMCache[scriptId]; ok {
		if vm == nil {
			return nil, errNoScript
		}
		return vm, nil
	}

	script := sItem.getScript()
	if len(script) == 0 {
		itemVMCache[scriptId] = nil
		return nil, errNoScript
	}

	vm := goja.New()
	setAllScriptingFunctions(vm)

	prg, err := goja.Compile(fmt.Sprintf(`item-%s`, scriptId), script, false)
	if err != nil {
		finalErr := fmt.Errorf("Compile: %w", err)
		return nil, finalErr
	}

	//
	// Run the program
	//
	tmr := time.AfterFunc(scriptLoadTimeout, func() {
		vm.Interrupt(errTimeout)
	})
	if _, err = vm.RunProgram(prg); err != nil {

		// Wrap the error
		finalErr := fmt.Errorf("RunProgram: %w", err)

		if _, ok := finalErr.(*goja.Exception); ok {
			slog.Error("JSVM", "exception", finalErr)
			return nil, finalErr
		} else if errors.Is(finalErr, errTimeout) {
			slog.Error("JSVM", "interrupted", finalErr)
			return nil, finalErr
		}

		slog.Error("JSVM", "error", finalErr)
		return nil, finalErr
	}
	vm.ClearInterrupt()
	tmr.Stop()

	vmw := newVMWrapper(vm, 0)

	itemVMCache[scriptId] = vmw

	return vmw, nil
}