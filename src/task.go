package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	"github.com/gen2brain/dlgs"
	"github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	TASK_TYPE_BOOLEAN = iota
	TASK_TYPE_PROGRESSBAR
	TASK_TYPE_NOTE
	TASK_TYPE_IMAGE
	TASK_TYPE_SOUND
)

type Task struct {
	Rect         rl.Rectangle
	Project      *Project
	Position     rl.Vector2
	PrevPosition rl.Vector2
	Open         bool
	Selected     bool
	MinSize      rl.Vector2

	TaskType    *Spinner
	Description *Textbox

	CompletionCheckbox    *Checkbox
	CompletionProgressbar *ProgressBar
	Image                 rl.Texture2D

	SoundControl  *beep.Ctrl
	SoundStream   beep.StreamSeekCloser
	SoundComplete bool
	FilePath      string
	PrevFilePath  string
	// ImagePathIsURL  // I don't know about the utility of this one. It's got cool points, though.
	ImageDisplaySize rl.Vector2
	Resizeable       bool
	Resizing         bool
	Dragging         bool

	TaskAbove           *Task
	TaskBelow           *Task
	OriginalIndentation int
	NumberingPrefix     []int
	RefreshPrefix       bool
	ID                  int
}

var taskID = 0

func NewTask(project *Project) *Task {
	task := &Task{
		Rect:                  rl.Rectangle{0, 0, 16, 16},
		Project:               project,
		TaskType:              NewSpinner(140, 32, 192, 16, "Check Box", "Progress Bar", "Note", "Image", "Sound"),
		Description:           NewTextbox(140, 64, 256, 64),
		CompletionCheckbox:    NewCheckbox(140, 96, 16, 16),
		CompletionProgressbar: NewProgressBar(140, 96, 192, 16),
		NumberingPrefix:       []int{-1},
		RefreshPrefix:         false,
		ID:                    project.GetFirstFreeID(),
	}
	task.MinSize = rl.Vector2{task.Rect.Width, task.Rect.Height}
	task.Description.AllowNewlines = true
	return task
}

func (task *Task) Clone() *Task {
	copyData := *task // By de-referencing and then making another reference, we should be essentially copying the struct

	desc := *copyData.Description
	copyData.Description = &desc

	tt := *copyData.TaskType
	// tt.Options = []string{}		// THIS COULD be a problem later; don't do anything about it if it's not necessary.
	// for _, opt := range task.TaskType.Options {

	// }
	copyData.TaskType = &tt

	cc := *copyData.CompletionCheckbox
	copyData.CompletionCheckbox = &cc

	cp := *copyData.CompletionProgressbar
	copyData.CompletionProgressbar = &cp

	copyData.SoundControl = nil
	copyData.SoundStream = nil

	copyData.ReceiveMessage("task close", nil) // We do this to recreate the sound file for the Task, if necessary.

	return &copyData
}

func (task *Task) Serialize() map[string]interface{} {

	data := map[string]interface{}{}
	data["Position.X"] = task.Position.X
	data["Position.Y"] = task.Position.Y
	data["Rect.W"] = task.Rect.Width
	data["Rect.H"] = task.Rect.Height
	data["ImageDisplaySize.X"] = task.ImageDisplaySize.X
	data["ImageDisplaySize.Y"] = task.ImageDisplaySize.Y
	data["Checkbox.Checked"] = task.CompletionCheckbox.Checked
	data["Progressbar.Percentage"] = task.CompletionProgressbar.Percentage
	data["Description"] = task.Description.Text
	data["FilePath"] = task.FilePath
	data["Selected"] = task.Selected
	data["TaskType.CurrentChoice"] = task.TaskType.CurrentChoice
	return data

}

func (task *Task) Deserialize(data map[string]interface{}) {

	// JSON encodes all numbers as 64-bit floats, so this saves us some visual ugliness.
	getFloat := func(name string) float32 {
		return float32(data[name].(float64))
	}
	getInt := func(name string) int32 {
		return int32(data[name].(float64))
	}

	task.Position.X = getFloat("Position.X")
	task.Position.Y = getFloat("Position.Y")
	task.Rect.X = task.Position.X
	task.Rect.Y = task.Position.Y
	task.Rect.Width = getFloat("Rect.W")
	task.Rect.Height = getFloat("Rect.H")
	task.ImageDisplaySize.X = getFloat("ImageDisplaySize.X")
	task.ImageDisplaySize.Y = getFloat("ImageDisplaySize.Y")
	task.CompletionCheckbox.Checked = data["Checkbox.Checked"].(bool)
	task.CompletionProgressbar.Percentage = getInt("Progressbar.Percentage")
	task.Description.Text = data["Description"].(string)
	task.FilePath = data["FilePath"].(string)
	task.PrevFilePath = task.FilePath
	task.Selected = data["Selected"].(bool)
	task.TaskType.CurrentChoice = int(data["TaskType.CurrentChoice"].(float64))

	// We do this to update the task after loading all of the information.
	task.ReceiveMessage("task close", map[string]interface{}{"task": task})
}

func (task *Task) Update() {

	task.SetPrefix()

	if task.SoundComplete {

		task.SoundComplete = false
		task.SoundControl.Paused = true
		task.SoundStream.Seek(0)
		speaker.Play(beep.Seq(task.SoundControl, beep.Callback(task.OnSoundCompletion)))

		if task.TaskBelow != nil && task.TaskBelow.TaskType.CurrentChoice == TASK_TYPE_SOUND && task.TaskBelow.SoundControl != nil {
			speaker.Lock()
			task.SoundControl.Paused = true
			task.TaskBelow.SoundControl.Paused = false
			speaker.Unlock()
		} else if task.TaskAbove != nil {

			above := task.TaskAbove
			for above.TaskAbove != nil && above.TaskAbove.SoundControl != nil && above.TaskAbove.TaskType.CurrentChoice == TASK_TYPE_SOUND {
				above = above.TaskAbove
			}

			if above != nil {
				speaker.Lock()
				task.SoundControl.Paused = true
				above.SoundControl.Paused = false
				speaker.Unlock()
			}
		} else {
			speaker.Lock()
			task.SoundControl.Paused = false
			speaker.Unlock()
		}

	}

	if task.Selected && task.Dragging && !task.Resizing {

		task.Position.X += GetMouseDelta().X
		task.Position.Y += GetMouseDelta().Y

	}

	if !task.Dragging || task.Resizing {

		if math.Abs(float64(task.Rect.X-task.Position.X)) <= 1 {
			task.Rect.X = task.Position.X
		}

		if math.Abs(float64(task.Rect.Y-task.Position.Y)) <= 1 {
			task.Rect.Y = task.Position.Y
		}

	}

	task.Rect.X += (task.Position.X - task.Rect.X) * 0.2
	task.Rect.Y += (task.Position.Y - task.Rect.Y) * 0.2

	color := getThemeColor(GUI_INSIDE)

	if task.IsComplete() {
		color = getThemeColor(GUI_INSIDE_HIGHLIGHTED)
	}

	if task.TaskType.CurrentChoice == TASK_TYPE_NOTE {
		color = getThemeColor(GUI_NOTE_COLOR)
	}

	outlineColor := getThemeColor(GUI_OUTLINE)

	if task.Selected {
		outlineColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
	}

	if task.Completable() {

		glowYPos := -task.Rect.Y / float32(task.Project.GridSize)
		glowXPos := -task.Rect.X / float32(task.Project.GridSize)
		glowVariance := float64(20)
		if task.Selected {
			glowVariance = 80
		}
		glow := uint8(math.Sin(float64((rl.GetTime()*math.Pi*2+glowYPos+glowXPos)))*(glowVariance/2) + (glowVariance / 2))

		if color.R >= glow {
			color.R -= glow
		} else {
			color.R = 0
		}

		if color.G >= glow {
			color.G -= glow
		} else {
			color.G = 0
		}

		if color.B >= glow {
			color.B -= glow
		} else {
			color.B = 0
		}

		if outlineColor.R >= glow {
			outlineColor.R -= glow
		} else {
			outlineColor.R = 0
		}

		if outlineColor.G >= glow {
			outlineColor.G -= glow
		} else {
			outlineColor.G = 0
		}

		if outlineColor.B >= glow {
			outlineColor.B -= glow
		} else {
			outlineColor.B = 0
		}

	}

	shadowRect := task.Rect
	shadowRect.X += 4
	shadowRect.Y += 2
	shadow := rl.Black
	shadow.A = color.A / 3
	rl.DrawRectangleRec(shadowRect, shadow)

	rl.DrawRectangleRec(task.Rect, color)
	if task.TaskType.CurrentChoice == TASK_TYPE_PROGRESSBAR && task.CompletionProgressbar.Percentage < 100 {
		c := getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
		r := task.Rect
		r.Width *= float32(task.CompletionProgressbar.Percentage) / 100
		c.A = color.A / 3
		rl.DrawRectangleRec(r, c)
	}

	if task.SoundControl != nil {
		pos := task.SoundStream.Position()
		len := task.SoundStream.Len()

		c := getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
		r := task.Rect
		r.Width *= float32(pos) / float32(len)
		c.A = color.A / 3
		rl.DrawRectangleRec(r, c)

	}

	rl.DrawRectangleLinesEx(task.Rect, 1, outlineColor)

	if task.TaskType.CurrentChoice == TASK_TYPE_IMAGE && task.Image.ID != 0 {

		src := rl.Rectangle{0, 0, float32(task.Image.Width), float32(task.Image.Height)}
		dst := task.Rect
		dst.Width = task.ImageDisplaySize.X
		dst.Height = task.ImageDisplaySize.Y
		rl.DrawTexturePro(task.Image, src, dst, rl.Vector2{}, 0, rl.White)
		// rl.DrawTexture(task.Image, int32(task.Rect.X), int32(task.Rect.Y), rl.White)

		if task.Resizeable && task.Selected {
			rec := task.Rect
			rec.Width = 8
			rec.Height = 8
			rec.X += task.Rect.Width
			rec.Y += task.Rect.Height
			rl.DrawRectangleRec(rec, getThemeColor(GUI_INSIDE))
			rl.DrawRectangleLinesEx(rec, 1, getThemeColor(GUI_FONT_COLOR))
			if rl.IsMouseButtonDown(rl.MouseLeftButton) && rl.CheckCollisionPointRec(GetWorldMousePosition(), rec) {
				task.Resizing = true
			} else if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
				task.Resizing = false
			}
			if task.Resizing {
				endPoint := GetWorldMousePosition()
				task.ImageDisplaySize.X = endPoint.X - task.Rect.X
				task.ImageDisplaySize.Y = endPoint.Y - task.Rect.Y
				if task.ImageDisplaySize.X < task.MinSize.X {
					task.ImageDisplaySize.X = task.MinSize.X
				}
				if task.ImageDisplaySize.Y < task.MinSize.Y {
					task.ImageDisplaySize.Y = task.MinSize.Y
				}
			}

			rec.X = task.Rect.X - rec.Width
			rec.Y = task.Rect.Y - rec.Height

			rl.DrawRectangleRec(rec, getThemeColor(GUI_INSIDE))
			rl.DrawRectangleLinesEx(rec, 1, getThemeColor(GUI_FONT_COLOR))

			if rl.IsMouseButtonPressed(rl.MouseLeftButton) && rl.CheckCollisionPointRec(GetWorldMousePosition(), rec) {
				task.ImageDisplaySize.X = float32(task.Image.Width)
				task.ImageDisplaySize.Y = float32(task.Image.Height)
			}

		}

	}

	name := task.Description.Text

	if task.TaskType.CurrentChoice == TASK_TYPE_IMAGE {
		name = ""
		task.Resizeable = true
	} else if task.TaskType.CurrentChoice == TASK_TYPE_SOUND {
		_, filename := path.Split(task.FilePath)
		name = filename
	} else if task.TaskType.CurrentChoice != TASK_TYPE_NOTE {
		// Notes don't get just the first line written on the task in the overview.
		cut := strings.Index(name, "\n")
		if cut >= 0 {
			name = name[:cut] + "[...]"
		}
		task.Resizeable = false
	}

	if task.NumberingPrefix[0] != -1 && task.Completable() {
		n := ""
		for _, value := range task.NumberingPrefix {
			n += fmt.Sprintf("%d.", value)
		}
		name = fmt.Sprintf("%s %s", n, name)
	}

	taskDisplaySize := rl.MeasureTextEx(font, name, fontSize, spacing)
	// Lock the sizes of the task to a grid
	taskDisplaySize.X = float32((math.Ceil(float64((taskDisplaySize.X + 4) / float32(task.Project.GridSize))))) * float32(task.Project.GridSize)
	taskDisplaySize.Y = float32((math.Ceil(float64((taskDisplaySize.Y + 4) / float32(task.Project.GridSize))))) * float32(task.Project.GridSize)
	task.Rect.Width = taskDisplaySize.X
	task.Rect.Height = taskDisplaySize.Y

	if task.Image.ID != 0 && task.TaskType.CurrentChoice == TASK_TYPE_IMAGE {
		if task.Rect.Width < task.ImageDisplaySize.X {
			task.Rect.Width = task.ImageDisplaySize.X
		}
		if task.Rect.Height < task.ImageDisplaySize.Y {
			task.Rect.Height = task.ImageDisplaySize.Y
		}
	}

	if task.Rect.Width < task.MinSize.X {
		task.Rect.Width = task.MinSize.X
	}
	if task.Rect.Height < task.MinSize.Y {
		task.Rect.Height = task.MinSize.Y
	}

	rl.DrawTextEx(font, name, rl.Vector2{task.Rect.X + 2, task.Rect.Y + 2}, fontSize, spacing, getThemeColor(GUI_FONT_COLOR))

}

func (task *Task) PostDraw() {

	if task.Open {

		rect := rl.Rectangle{16, 16, screenWidth - 32, screenHeight - 32}

		rl.DrawRectangleRec(rect, getThemeColor(GUI_INSIDE))
		rl.DrawRectangleLinesEx(rect, 1, getThemeColor(GUI_OUTLINE))

		raygui.Label(rl.Rectangle{rect.X, task.TaskType.Rect.Y, 0, 16}, "Task Type: ")
		task.TaskType.Update()

		y := task.TaskType.Rect.Y + 16

		if task.TaskType.CurrentChoice != TASK_TYPE_IMAGE && task.TaskType.CurrentChoice != TASK_TYPE_SOUND {
			task.Description.Update()
			raygui.Label(rl.Rectangle{rect.X, task.Description.Rect.Y + 8, 0, 16}, "Description: ")
			y += task.Description.Rect.Height + 16
		}

		if ImmediateButton(rl.Rectangle{rect.Width, rect.Y, 16, 16}, "X", false) {
			task.Open = false
			task.Project.TaskOpen = false
			task.Project.SendMessage("task close", map[string]interface{}{"task": task})
		}

		if task.TaskType.CurrentChoice == TASK_TYPE_BOOLEAN {
			raygui.Label(rl.Rectangle{rect.X, y + 8, 0, 0}, "Completed: ")
			task.CompletionCheckbox.Rect.Y = y + 8
			task.CompletionCheckbox.Update()
		} else if task.TaskType.CurrentChoice == TASK_TYPE_PROGRESSBAR {
			raygui.Label(rl.Rectangle{rect.X, y + 8, 0, 0}, "Percentage: ")
			task.CompletionProgressbar.Rect.Y = y + 8
			task.CompletionProgressbar.Update()
		} else if task.TaskType.CurrentChoice == TASK_TYPE_IMAGE {
			imagePath := "Image File: "
			if task.FilePath == "" {
				imagePath += "[None]"
			} else {
				imagePath += task.FilePath
			}
			raygui.Label(rl.Rectangle{rect.X, y + 8, 0, 0}, imagePath)
			if ImmediateButton(rl.Rectangle{rect.X + 16, y + 32, 64, 16}, "Load", false) {
				//rl.HideWindow()	// Not with the old version of Raylib that raylib-go ships with :/
				filepath, success, _ := dlgs.File("Load Image", "Image Files | *.png *.jpg *.bmp *.tiff", false)
				if success {
					task.FilePath = filepath
				}
				//rl.ShowWindow()
			}
			if ImmediateButton(rl.Rectangle{rect.X + 96, y + 32, 64, 16}, "Clear", false) {
				task.FilePath = ""
			}
		} else if task.TaskType.CurrentChoice == TASK_TYPE_SOUND {
			imagePath := "Sound File: "
			if task.FilePath == "" {
				imagePath += "[None]"
			} else {
				imagePath += task.FilePath
			}
			raygui.Label(rl.Rectangle{rect.X, y + 8, 0, 0}, imagePath)
			if ImmediateButton(rl.Rectangle{rect.X + 16, y + 32, 64, 16}, "Load", false) {
				//rl.HideWindow()	// Not with the old version of Raylib that raylib-go ships with :/
				filepath, success, _ := dlgs.File("Load Sound", "Sound Files | *.wav *.ogg *.xm *.mod *.flac *.mp3", false)
				if success {
					task.FilePath = filepath
				}
				//rl.ShowWindow()
			}
			if ImmediateButton(rl.Rectangle{rect.X + 96, y + 32, 64, 16}, "Clear", false) {
				task.FilePath = ""
			}
		}

		y += 48

	}

}

func (task *Task) IsComplete() bool {
	if task.TaskType.CurrentChoice == TASK_TYPE_BOOLEAN {
		return task.CompletionCheckbox.Checked
	} else if task.TaskType.CurrentChoice == TASK_TYPE_PROGRESSBAR {
		return task.CompletionProgressbar.Percentage == 100
	}
	return false
}

func (task *Task) Completable() bool {
	return task.TaskType.CurrentChoice == TASK_TYPE_BOOLEAN || task.TaskType.CurrentChoice == TASK_TYPE_PROGRESSBAR
}

func (task *Task) CanHaveNeighbors() bool {
	return task.TaskType.CurrentChoice != TASK_TYPE_IMAGE && task.TaskType.CurrentChoice != TASK_TYPE_NOTE
}

func (task *Task) ToggleCompletion() {

	if task.Completable() {

		task.CompletionCheckbox.Checked = !task.CompletionCheckbox.Checked

		if task.CompletionProgressbar.Percentage != 100 {
			task.CompletionProgressbar.Percentage = 100
		} else {
			task.CompletionProgressbar.Percentage = 0
		}

	} else if task.TaskType.CurrentChoice == TASK_TYPE_SOUND {
		// task.ReceiveMessage("dropped", nil) // Play the sound
		task.ToggleSound()
	}

}

func (task *Task) ReceiveMessage(message string, data map[string]interface{}) {

	if message == "select" {

		if data["task"] == task {
			task.Selected = true
		} else if data["task"] == nil || data["task"] != task {
			task.Selected = false
		}
	} else if message == "deselect" {
		task.Selected = false
	} else if message == "double click" {
		task.Open = true
		task.Project.SendMessage("task open", nil)
		task.Project.TaskOpen = true
		task.Dragging = false
	} else if message == "task close" {
		if task.FilePath != "" {
			if task.TaskType.CurrentChoice == TASK_TYPE_IMAGE {
				if task.Image.ID > 0 {
					rl.UnloadTexture(task.Image)
				}
				task.Image = rl.LoadTexture(task.FilePath)
				if task.PrevFilePath != task.FilePath {
					task.ImageDisplaySize.X = float32(task.Image.Width)
					task.ImageDisplaySize.Y = float32(task.Image.Height)
				}
			} else if task.TaskType.CurrentChoice == TASK_TYPE_SOUND {

				file, err := os.Open(task.FilePath)
				if err != nil {
					log.Println("ERROR: Could not load file: ", task.FilePath)
				} else {

					if task.SoundStream != nil {
						task.SoundStream.Close()
						task.SoundStream = nil
						task.SoundControl = nil
					}

					ext := strings.ToLower(path.Ext(task.FilePath))
					var stream beep.StreamSeekCloser
					var format beep.Format
					var err error

					if strings.Contains(ext, "mp3") {
						stream, format, err = mp3.Decode(file)
					} else if strings.Contains(ext, "ogg") {
						stream, format, err = vorbis.Decode(file)
					} else if strings.Contains(ext, "flac") {
						stream, format, err = flac.Decode(file)
					} else {
						// Going to assume it's a WAV
						stream, format, err = wav.Decode(file)
					}

					if err != nil {
						log.Println("ERROR: Could not decode file: ", task.FilePath)
						log.Println(err)
					} else {
						task.SoundStream = stream

						if format.SampleRate != task.Project.SampleRate {
							log.Println("Sample rate of audio file", task.FilePath, "not the same as project sample rate.")
							log.Println("File will be resampled.")
							resampled := beep.Resample(1, format.SampleRate, 44100, stream)
							task.SoundControl = &beep.Ctrl{Streamer: resampled, Paused: true}
						} else {
							task.SoundControl = &beep.Ctrl{Streamer: stream, Paused: true}
						}
						speaker.Play(beep.Seq(task.SoundControl, beep.Callback(task.OnSoundCompletion)))
					}

				}

			}
			task.PrevFilePath = task.FilePath
		}
	} else if message == "dragging" {
		task.Dragging = task.Selected
	} else if message == "dropped" {
		task.Dragging = false
		task.Position.X, task.Position.Y = task.Project.LockPositionToGrid(task.Position.X, task.Position.Y)
		task.GetNeighbors()
		task.RefreshPrefix = true
		// If you didn't move, this was a click, not a drag and drop
		if task.Selected && task.Position == task.PrevPosition && task.TaskType.CurrentChoice == TASK_TYPE_SOUND && task.SoundControl != nil {
			task.ToggleSound()
		}
		task.PrevPosition = task.Position

	} else if message == "delete" {

		if data["task"] == task {
			if task.SoundStream != nil {
				task.SoundStream.Close()
				task.SoundControl.Paused = true
				task.SoundControl = nil
			}
			if task.Image.ID > 0 {
				rl.UnloadTexture(task.Image)
			}
		}

	}

}

func (task *Task) ToggleSound() {
	if task.SoundControl != nil {
		speaker.Lock()
		task.SoundControl.Paused = !task.SoundControl.Paused
		speaker.Unlock()
	}
}

func (task *Task) StopSound() {
	if task.SoundControl != nil {
		speaker.Lock()
		task.SoundControl.Paused = true
		speaker.Unlock()
	}
}

func (task *Task) OnSoundCompletion() {
	task.SoundComplete = true
}

func (task *Task) GetNeighbors() {

	if !task.CanHaveNeighbors() {
		return
	}

	for _, other := range task.Project.Tasks {
		if other != task && other.CanHaveNeighbors() {

			taskRec := task.Rect
			taskRec.X = task.Position.X
			taskRec.Y = task.Position.Y + 8 // Before this offset was just 1, but that
			// created a bug allowing you to drag down a bit and break the neighboring somehow

			otherRec := other.Rect
			otherRec.X = other.Position.X
			otherRec.Y = other.Position.Y

			if rl.CheckCollisionRecs(taskRec, otherRec) && (taskRec.X != otherRec.X || taskRec.Y-8 != otherRec.Y) {
				if other.TaskBelow != task {
					other.TaskAbove = task
				}
				if task.TaskAbove != other {
					task.TaskBelow = other
				}
				break
			}

		}
	}
}

func (task *Task) SetPrefix() {

	if task.RefreshPrefix {

		if task.TaskAbove != nil {

			task.NumberingPrefix = append([]int{}, task.TaskAbove.NumberingPrefix...)

			above := task.TaskAbove
			if above.Position.X < task.Position.X {
				task.NumberingPrefix = append(task.NumberingPrefix, 0)
			} else if above.Position.X > task.Position.X {
				d := len(above.NumberingPrefix) - int((above.Position.X-task.Position.X)/float32(task.Project.GridSize))
				if d < 1 {
					d = 1
				}

				task.NumberingPrefix = append([]int{}, above.NumberingPrefix[:d]...)
			}

			task.NumberingPrefix[len(task.NumberingPrefix)-1] += 1

		} else if task.TaskBelow != nil {
			task.NumberingPrefix = []int{1}
		} else {
			task.NumberingPrefix = []int{-1}
		}

		task.RefreshPrefix = false

	}

}
