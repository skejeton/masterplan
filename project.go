package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/ncruces/zenity"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/veandco/go-sdl2/sdl"
)

// const (
// 	SequenceNumber = iota
// 	SequenceNumberDash
// 	SequenceRoman
// 	SequenceBullet
// 	SequenceOff
// )

// const (
// 	SettingsGeneral = iota
// 	SettingsTasks
// 	SettingsGlobal
// 	SettingsKeyboard
// 	SettingsAbout
// )

const (
	GUIFontSize100 = "100%"
	GUIFontSize150 = "150%"
	GUIFontSize200 = "200%"
	GUIFontSize250 = "250%"
	GUIFontSize300 = "300%"
	GUIFontSize350 = "350%"
	GUIFontSize400 = "400%"

	// Project actions

	ActionNewProject    = "new"
	ActionLoadProject   = "load"
	ActionSaveAsProject = "save as"
	ActionRenameBoard   = "rename"
	ActionQuit          = "quit"

	BackupDelineator = "_bak_"
	FileTimeFormat   = "01_02_06_15_04_05"
)

type Project struct {
	ProjectSettings  *ProjectSettings
	Pages            []*Page
	CurrentPageIndex int
	Camera           *Camera
	GridTexture      *Image
	Filepath         string
	LoadingProject   *Project // A reference to the "next" Project when opening another one
	UndoHistory      *UndoHistory
}

func NewProject() *Project {

	project := &Project{
		ProjectSettings: NewProjectSettings(),
		Camera:          NewCamera(),
		Pages:           []*Page{},
		UndoHistory:     NewUndoHistory(),
	}

	project.Pages = append(project.Pages, NewPage(project))

	guiTex := globals.Resources.Get("assets/gui.png").AsImage()

	gridColor := getThemeColor(GUIGridColor)
	guiTex.Texture.SetColorMod(gridColor.RGB())
	guiTex.Texture.SetAlphaMod(gridColor[3])

	project.GridTexture = TileTexture(guiTex, &sdl.Rect{480, 0, 32, 32}, 512, 512)

	return project

}

func (project *Project) Update() {

	globals.Mouse.HiddenPosition = false

	globals.Mouse.ApplyCursor()

	globals.Mouse.SetCursor("normal")

	for _, page := range project.Pages {
		page.Update()
	}

	globals.Mouse.HiddenPosition = false

	project.GlobalShortcuts()

	project.MouseActions()

	globals.InputText = []rune{}

	project.UndoHistory.Update()

	project.Camera.Update()

}

func (project *Project) Draw() {

	drawGridPiece := func(x, y float32) {
		globals.Renderer.CopyF(project.GridTexture.Texture, nil, &sdl.FRect{x, y, project.GridTexture.Size.X, project.GridTexture.Size.Y})
	}

	if project.Camera.Zoom > 0.5 {

		extent := float32(10)
		for y := -extent; y < extent; y++ {
			for x := -extent; x < extent; x++ {
				translated := project.Camera.TranslateRect(&sdl.FRect{x * project.GridTexture.Size.X, y * project.GridTexture.Size.Y, 0, 0})
				drawGridPiece(translated.X, translated.Y)
			}
		}

	}

	// gridPieceToScreenW := globals.ScreenSize.X / project.GridTexture.Size.X / project.Camera.TargetZoom
	// gridPieceToScreenH := globals.ScreenSize.Y / project.GridTexture.Size.Y / project.Camera.TargetZoom

	// for iy := -gridPieceToScreenH; iy < gridPieceToScreenH; iy++ {
	// 	for ix := -gridPieceToScreenW; ix < gridPieceToScreenW; ix++ {

	// 		x := float32(ix * project.GridTexture.Size.X)
	// 		x += float32(math.Round(float64(project.Camera.Position.X / project.GridTexture.Size.X * project.GridTexture.Size.X)))

	// 		y := float32(iy * project.GridTexture.Size.Y)
	// 		y += float32(math.Round(float64(project.Camera.Position.Y / project.GridTexture.Size.Y * project.GridTexture.Size.Y)))

	// 		// x -= int32(project.Camera.Position.X)

	// 		translated := project.Camera.Translate(&sdl.FRect{x, y, 0, 0})

	// 		drawGridPiece(translated.X, translated.Y)

	// 	}
	// }

	project.CurrentPage().Draw()

}

func (project *Project) Save() {

	saveData, _ := sjson.Set("{}", "version", globals.Version.String())

	for _, page := range project.Pages {
		saveData, _ = sjson.SetRaw(saveData, "pages.-1", page.Serialize())
	}

	saveData = gjson.Get(saveData, "@pretty").String()

	if file, err := os.Create(project.Filepath); err != nil {
		log.Println(err)
	} else {
		file.Write([]byte(saveData))
		file.Close()
	}

}

func (project *Project) SaveAs() {

	if filename, err := zenity.SelectFileSave(zenity.Title("Save MasterPlan Project..."), zenity.ConfirmOverwrite(), zenity.FileFilter{Name: "Project File (*.plan)", Patterns: []string{"*.plan"}}); err == nil {

		if filepath.Ext(filename) != ".plan" {
			filename += ".plan"
		}

		project.Filepath = filename

		project.Save()

	} else if err != zenity.ErrCanceled {
		panic(err)
	}

}

// Open a project to load
func (project *Project) Open() {

	if filename, err := zenity.SelectFile(zenity.Title("Select MasterPlan Project to Open..."), zenity.FileFilter{Name: "Project File (*.plan)", Patterns: []string{"*.plan"}}); err == nil {

		jsonData, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}

		newProject := NewProject()

		for i, page := range gjson.Get(string(jsonData), "pages").Array() {
			var newPage *Page
			if i == 0 {
				newPage = newProject.Pages[0]
			} else {
				newPage = NewPage(newProject)
				newProject.Pages = append(newProject.Pages, newPage)
			}
			newPage.Deserialize(page.Raw)
		}

		project.LoadingProject = newProject

	} else if err != zenity.ErrCanceled {
		panic(err)
	}

}

func (project *Project) Destroy() {

}

func (project *Project) MouseActions() {

	if globals.State == StateNeutral {

		if globals.Mouse.Button(sdl.BUTTON_LEFT).PressedTimes(2) {

			globals.Mouse.Button(sdl.BUTTON_LEFT).Consume()
			card := project.CurrentPage().CreateNewCard(ContentTypeCheckbox)
			card.Rect.X = globals.Mouse.WorldPosition().X - (card.Rect.W / 2)
			card.Rect.Y = globals.Mouse.WorldPosition().Y - (card.Rect.H / 2)

			card.LockPosition()

		}

		if globals.Mouse.Button(sdl.BUTTON_RIGHT).Pressed() {
			contextMenu := globals.MenuSystem.Get("context")
			contextMenu.Rect.X = globals.Mouse.Position().X
			contextMenu.Rect.Y = globals.Mouse.Position().Y
			contextMenu.Open()
		}

	}

	if globals.State != StateContextMenu {

		if globals.Mouse.Wheel() > 0 {
			project.Camera.AddZoom(0.25)
		} else if globals.Mouse.Wheel() < 0 {
			project.Camera.AddZoom(-0.25)
		}

		if globals.Mouse.Button(sdl.BUTTON_MIDDLE).Held() {
			project.Camera.TargetPosition = project.Camera.TargetPosition.Sub(globals.Mouse.RelativeMovement().Mult(8))
		}

	}

}

func (project *Project) SendMessage(msg *Message) {

	if msg.Type == "" {
		panic("ERROR: Message has no type.")
	}

	for _, page := range project.Pages {
		page.SendMessage(msg)
	}

}

func (project *Project) GlobalShortcuts() {

	if globals.State == StateNeutral {

		dx := float32(0)
		dy := float32(0)
		panSpeed := float32(8)

		if globals.ProgramSettings.Keybindings.On(KBPanRight) {
			dx = panSpeed
		}
		if globals.ProgramSettings.Keybindings.On(KBPanLeft) {
			dx = -panSpeed
		}

		if globals.ProgramSettings.Keybindings.On(KBPanUp) {
			dy = -panSpeed
		}
		if globals.ProgramSettings.Keybindings.On(KBPanDown) {
			dy = panSpeed
		}

		if globals.ProgramSettings.Keybindings.On(KBFastPan) {
			dx *= 2
			dy *= 2
		}

		project.Camera.TargetPosition.X += dx / project.Camera.Zoom
		project.Camera.TargetPosition.Y += dy / project.Camera.Zoom

		if globals.ProgramSettings.Keybindings.On(KBZoomIn) {
			project.Camera.AddZoom(1)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomOut) {
			project.Camera.AddZoom(-1)
		}

		if globals.ProgramSettings.Keybindings.On(KBZoomLevel25) {
			project.Camera.SetZoom(0.25)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomLevel50) {
			project.Camera.SetZoom(0.5)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomLevel100) {
			project.Camera.SetZoom(1.0)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomLevel200) {
			project.Camera.SetZoom(2.0)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomLevel400) {
			project.Camera.SetZoom(4.0)
		} else if globals.ProgramSettings.Keybindings.On(KBZoomLevel1000) {
			project.Camera.SetZoom(10.0)
		}

		if globals.ProgramSettings.Keybindings.On(KBReturnToOrigin) {
			project.Camera.TargetPosition = Point{}
		}

		if globals.ProgramSettings.Keybindings.On(KBNewCheckboxCard) {
			project.CurrentPage().CreateNewCard(ContentTypeCheckbox)
		} else if globals.ProgramSettings.Keybindings.On(KBNewNoteCard) {
			project.CurrentPage().CreateNewCard(ContentTypeNote)
		} else if globals.ProgramSettings.Keybindings.On(KBNewSoundCard) {
			project.CurrentPage().CreateNewCard(ContentTypeSound)
		} else if globals.ProgramSettings.Keybindings.On(KBNewImageCard) {
			project.CurrentPage().CreateNewCard(ContentTypeImage)
		} else if globals.ProgramSettings.Keybindings.On(KBNewTimerCard) {
			project.CurrentPage().CreateNewCard(ContentTypeTimer)
		}

		if globals.ProgramSettings.Keybindings.On(KBDeleteCards) {
			project.CurrentPage().DeleteCards(project.CurrentPage().Selection.AsSlice()...)
		}

		if globals.ProgramSettings.Keybindings.On(KBSelectAllCards) {
			for _, card := range project.CurrentPage().Cards {
				// card.Select()
				project.CurrentPage().Selection.Add(card)
			}

		}

		if globals.ProgramSettings.Keybindings.On(KBCopyCards) {
			project.CurrentPage().CopySelectedCards()
		}

		if globals.ProgramSettings.Keybindings.On(KBPasteCards) {

			project.CurrentPage().PasteCards()

		}

		if globals.ProgramSettings.Keybindings.On(KBSaveProject) {

			if project.Filepath != "" {
				project.Save()
			} else {
				project.SaveAs()
			}

		} else if globals.ProgramSettings.Keybindings.On(KBSaveProjectAs) {
			project.SaveAs()
		} else if globals.ProgramSettings.Keybindings.On(KBOpenProject) {
			project.Open()
		}

		if globals.ProgramSettings.Keybindings.On(KBUndo) {
			project.UndoHistory.Undo()
		} else if globals.ProgramSettings.Keybindings.On(KBRedo) {
			project.UndoHistory.Redo()
		}

	}

}

func (project *Project) CurrentPage() *Page {
	return project.Pages[project.CurrentPageIndex]
}
