package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
)

type GraphStyle int

const (
	graphStyleLine = iota
	graphStyleBar
)

var (
	gray      = color.RGBA{50, 50, 50, 255}
	darkGreen = color.RGBA{0, 100, 0, 255}
	red       = color.RGBA{255, 0, 0, 255}
)

func NewGraph(W, H int, FG, BG *color.RGBA, style GraphStyle) *Graph {
	return &Graph{
		icon:  image.NewRGBA(image.Rect(0, 0, W, H)),
		W:     W,
		H:     H,
		FG:    FG,
		BG:    BG,
		style: style,
	}
}

type Graph struct {
	icon  *image.RGBA
	W     int
	H     int
	FG    *color.RGBA
	BG    *color.RGBA
	style GraphStyle
}

func (g *Graph) Blank() {
	for x := 0; x < g.W; x++ {
		g.BlankVLine(x)
	}
}

func (g *Graph) BlankVLine(x int) {
	for y := 0; y < g.H; y++ {
		g.icon.Set(x, y, g.BG)
	}
}

func (g *Graph) SetNext(v int) {
	g.Scroll()
	g.VLine(v)
}

func (g *Graph) VLine(v int) {
	if v > g.H {
		log.Printf("VLine: value %d must be smaller than height %d, ignoring", v, g.H)
		return
	}
	// flip vertically. The input value expects 0 at the bottom, but the image
	// expects 0 at the top.
	v = g.H - v
	for y := 0; y < g.H; y++ {
		col := g.BG
		switch g.style {
		case graphStyleBar:
			if v <= y {
				col = g.FG
			}
		case graphStyleLine:
		default:
			if v == y {
				col = g.FG
			}
		}
		g.icon.Set(g.W-1, y, col)
	}
}

func (g *Graph) Scroll() {
	for x := 1; x < g.W; x++ {
		for y := 0; y < g.H; y++ {
			g.icon.Set(x-1, y, g.icon.At(x, y))
		}
	}
}

func (g *Graph) ToIcon() ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, g.icon, nil); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}
