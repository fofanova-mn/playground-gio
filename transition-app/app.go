package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"golang.org/x/exp/shiny/materialdesign/icons"

	ycwidget "github.com/yarcat/playground-gio/transition-app/widget"
)

type transitionApp struct {
	win           *app.Window
	theme         *material.Theme
	thumbnails    layout.List
	thumbnailImgs []*ycwidget.Image
	animations    []*FrameSet
}

func newTransitionApp(imgs ...image.Image) *transitionApp {
	thumbnails := make([]*ycwidget.Image, 0, len(imgs))
	animations := make([]*FrameSet, 0, len(imgs))
	for i, img := range imgs {
		thumbnails = append(thumbnails, ycwidget.NewImage(img))
		frames := 25
		duration := 100 * time.Millisecond
		var opts []FrameSetOptionFunc
		if i == 0 {
			opts = append(opts, ReversePlayback)
		}
		animations = append(animations, ApplyTransparency(img, frames, duration, opts...))
	}
	return &transitionApp{
		win:           app.NewWindow(),
		theme:         material.NewTheme(gofont.Collection()),
		thumbnailImgs: thumbnails,
		animations:    animations,
	}
}

type avState int

const (
	avStatePaused avState = iota
	avStatePlaying
)

var avIcons = [2]*widget.Icon{mustNewIcon(icons.AVPlayArrow), mustNewIcon(icons.AVPause)}

func (state avState) icon() *widget.Icon {
	return avIcons[state]
}

func (state avState) change() avState { return 1 - state }

type navState int

const (
	navStateForward navState = iota
	navStateBack
)

var navIcons = [2]*widget.Icon{
	mustNewIcon((icons.NavigationArrowForward)),
	mustNewIcon(icons.NavigationArrowBack),
}

var saveIcon = mustNewIcon(icons.ContentSave)

func (state navState) icon() *widget.Icon {
	return navIcons[state]
}

func (state navState) change() navState { return 1 - state }

func (app *transitionApp) mainloop() error {
	ops := &op.Ops{}

	thumbs := make(thumbnails, 0, len(app.thumbnailImgs))
	for _, img := range app.thumbnailImgs {
		thumbs = append(thumbs, &clickable{widget: img.Layout})
	}

	selected := 0

	var (
		avCtrl  widget.Clickable
		avState avState
	)

	var (
		navCtrl  widget.Clickable
		navState navState
	)

	var saveCtrl widget.Clickable

	for e := range app.win.Events() {
		switch e := e.(type) {
		case system.FrameEvent:
			gtx := layout.NewContext(ops, e)

			for i, c := range thumbs {
				if c.button.Clicked() {
					selected = i
				}
			}

			if avCtrl.Clicked() {
				avState = avState.change()
			}

			if navCtrl.Clicked() {
				navState = navState.change()
				if navState == navStateBack {
					avState = avStatePlaying
				}
			}

			axis := layout.Horizontal
			if gtx.Constraints.Max.X < gtx.Constraints.Max.Y {
				axis = layout.Vertical
			}
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layoutTab(gtx, app.theme, avState, &avCtrl, navState, &navCtrl, app.thumbnailImgs[selected], app.animations, axis),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: axis}.Layout(gtx, layoutButtons(gtx, app.theme, avState, &avCtrl, navState, &navCtrl, &saveCtrl)...)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.S.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return thumbs.Layout(gtx, &app.thumbnails, selected)
					})
				}),
			)

			e.Frame(gtx.Ops)
		case system.DestroyEvent:
			return e.Err
		}
	}
	return nil
}

func layoutButtons(
	gtx layout.Context,
	theme *material.Theme,
	avState avState,
	avCtrl *widget.Clickable,
	navState navState,
	navCtrl *widget.Clickable,
	saveCtrl *widget.Clickable) []layout.FlexChild {
	var children []layout.FlexChild

	children = append(children, layout.Flexed(1, material.IconButton(theme, navCtrl, navState.icon()).Layout))
	if navState == navStateBack {
		children = append(children, layout.Flexed(1, material.IconButton(theme, avCtrl, avState.icon()).Layout))
		children = append(children, layout.Flexed(1, material.IconButton(theme, saveCtrl, saveIcon).Layout))
	}
	return children
}

func layoutTab(
	gtx layout.Context,
	theme *material.Theme,
	avState avState,
	avCtrl *widget.Clickable,
	navState navState,
	navCtrl *widget.Clickable,
	selectedImage *ycwidget.Image,
	animations []*FrameSet,
	axis layout.Axis) layout.FlexChild {
	if navState == navStateForward {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, selectedImage.Layout)
		})
	}
	return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layoutTransformation(gtx, theme, avState, navState, selectedImage, animations),
		)
	})
}

func layoutTransformation(
	gtx layout.Context,
	theme *material.Theme,
	avState avState,
	navState navState,
	selectedImage *ycwidget.Image,
	animations []*FrameSet) layout.StackChild {
	if avState == avStatePaused {
		return layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			defer op.Record(gtx.Ops).Stop()
			return layout.Center.Layout(gtx, selectedImage.Layout)
		})
	}
	return layout.Expanded(func(gtx layout.Context) layout.Dimensions {
		var d layout.Dimensions
		for _, anim := range animations {
			d = anim.Layout(gtx)
		}
		return d
	})
}

const (
	thumbnailInsetDp = 10
	thumbnailSizePx  = 150
)

type thumbnails []*clickable

func (th thumbnails) Layout(gtx layout.Context, list *layout.List, selected int) layout.Dimensions {
	return list.Layout(gtx, len(th), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.UniformInset(unit.Dp(thumbnailInsetDp)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return th[index].Layout(gtx, selected == index)
		})
	})
}

type clickable struct {
	widget layout.Widget
	button widget.Clickable
}

func (btn *clickable) Layout(gtx layout.Context, selected bool) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	cornerRad := unit.Dp(10)
	d := material.Clickable(gtx, &btn.button, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max = image.Pt(thumbnailSizePx, thumbnailSizePx)
		if !selected {
			return btn.widget(gtx)
		}
		return widget.Border{
			Color:        color.RGBA{A: 0xff, R: 0x1f, G: 0x1f, B: 0x1f},
			Width:        unit.Dp(1),
			CornerRadius: cornerRad,
		}.Layout(gtx, btn.widget)
	})
	call := macro.Stop()

	defer op.Push(gtx.Ops).Pop()
	clip.RRect{
		Rect: f32.Rectangle{Max: layout.FPt(d.Size)},
		NW:   cornerRad.V * gtx.Metric.PxPerDp,
		NE:   cornerRad.V * gtx.Metric.PxPerDp,
		SW:   cornerRad.V * gtx.Metric.PxPerDp,
		SE:   cornerRad.V * gtx.Metric.PxPerDp,
	}.Add(gtx.Ops)
	call.Add(gtx.Ops)

	return d
}
