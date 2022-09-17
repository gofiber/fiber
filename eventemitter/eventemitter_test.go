package eventemitter

import (
	"sort"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
)

//TODO: add benchmark test

func Test_New_Emitter(t *testing.T) {
	emitter := New()
	utils.AssertEqual(t, emitter != nil, true)
}

func Test_AddListener(t *testing.T) {
	emitter := New()

	err := emitter.AddListener("event", func() {})
	utils.AssertEqual(t, nil, err)
	_, ok := emitter.listeners.Load("event")
	utils.AssertEqual(t, true, ok)
}

func Test_On(t *testing.T) {
	emitter := New()

	err := emitter.On("event", func() {})
	utils.AssertEqual(t, nil, err)
	_, ok := emitter.listeners.Load("event")
	utils.AssertEqual(t, true, ok)
}

func Test_Once(t *testing.T) {
	emitter := New()

	err := emitter.On("event", func() {})
	utils.AssertEqual(t, nil, err)
	_, ok := emitter.listeners.LoadAndDelete("event")
	utils.AssertEqual(t, true, ok)
}

func Test_RemoveListener(t *testing.T) {
	emitter := New()
	event := func() {}

	err := emitter.AddListener("event", &event)
	utils.AssertEqual(t, nil, err)
	_, ok := emitter.listeners.Load("event")
	utils.AssertEqual(t, true, ok)
	_, err = emitter.RemoveListener("event", &event)
	utils.AssertEqual(t, err, nil)
	_, ok = emitter.listeners.Load("event")
	utils.AssertEqual(t, ok, false)
}

func Test_Off(t *testing.T) {
	emitter := New()
	event := func() {}

	err := emitter.AddListener("event", &event)
	utils.AssertEqual(t, nil, err)
	_, ok := emitter.listeners.Load("event")
	utils.AssertEqual(t, true, ok)
	_, err = emitter.Off("event", &event)
	utils.AssertEqual(t, err, nil)
	_, ok = emitter.listeners.Load("event")
	utils.AssertEqual(t, ok, false)
}

func Test_Emit(t *testing.T) {
	emitter := New()

	emitter.On("fiber", func(message string) {
		utils.AssertEqual(t, message, "fiber is amazing")
	})

	emitter.On("anonymous", func() {
		isCalled := true
		utils.AssertEqual(t, isCalled, true)
	})

	err := emitter.Emit("fiber", "fiber is amazing")
	utils.AssertEqual(t, err, nil)

	// event is not exists
	err = emitter.Emit("unknow", "")
	utils.AssertEqual(t, err, ErrEventNotExists)

	// empty arguments
	err = emitter.Emit("anonymous")
	utils.AssertEqual(t, err, nil)
}

func Test_RemoveAllListeners(t *testing.T) {
	emitter := New()
	event := func() {}
	eventNames := []string{
		"event_1", "event_2", "event3",
	}

	for _, en := range eventNames {
		err := emitter.On(en, event)
		utils.AssertEqual(t, err, nil)
		_, ok := emitter.listeners.Load(en)
		utils.AssertEqual(t, ok, true)
	}

	emitter.RemoveAllListeners()
	utils.AssertEqual(t, 0, len(emitter.EventNames()))
}

func contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

func Test_EventNames(t *testing.T) {
	emitter := New()
	event := func() {}
	eventNames := []string{
		"event_1", "event_2", "event3",
	}

	for _, en := range eventNames {
		err := emitter.On(en, event)
		utils.AssertEqual(t, err, nil)
		_, ok := emitter.listeners.Load(en)
		utils.AssertEqual(t, ok, true)
	}

	utils.AssertEqual(t, contains(emitter.EventNames(), "event_1"), true)
	utils.AssertEqual(t, 3, len(emitter.EventNames()))
}

func Test_Listeners(t *testing.T) {
	emitter := New()
	event := func() {}
	eventNames := []string{
		"event_1", "event_1", "event_1", "event_2",
	}

	for _, en := range eventNames {
		err := emitter.On(en, &event)
		utils.AssertEqual(t, err, nil)
		_, ok := emitter.listeners.Load(en)
		utils.AssertEqual(t, ok, true)
	}

	listeners, _ := emitter.Listeners("event_1")
	utils.AssertEqual(t, 3, len(listeners))
}

func Test_ListenerCount(t *testing.T) {
	emitter := New()
	event := func() {}
	eventNames := []string{
		"event_1", "event_1", "event2",
	}

	for _, en := range eventNames {
		err := emitter.On(en, event)
		utils.AssertEqual(t, err, nil)
		_, ok := emitter.listeners.Load(en)
		utils.AssertEqual(t, ok, true)
	}

	count, _ := emitter.ListenerCount("event_1")

	utils.AssertEqual(t, 2, count)
}

func Test_Is_Empty_Event_Name(t *testing.T) {
	emitter := New()

	err := emitter.On("", func() {})
	utils.AssertEqual(t, ErrEmptyName, err)
}

func Test_Is_Empty_Event_Handler(t *testing.T) {
	emitter := New()

	err := emitter.On("event", nil)
	utils.AssertEqual(t, ErrNotAFunction, err)
}

func Test_Is_Not_A_Function(t *testing.T) {
	emitter := New()

	err := emitter.On("damn", "it is not a function")
	utils.AssertEqual(t, ErrNotAFunction, err)
}
