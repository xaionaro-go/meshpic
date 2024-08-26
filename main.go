package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"

	"github.com/spf13/pflag"
)

type nodeT struct {
	X           float64
	Y           float64
	DomainID    uint64
	IsInterface bool
}

// Ironically, this script was created to explain that
// code coupling is one of two worst things one can do
// with a code. But this code is a one-time-use, so
// everything is kept inside `main()` :)

func main() {
	width := pflag.Uint("width", 2020, "")
	height := pflag.Uint("height", 1180, "")
	nodesCount := pflag.Uint("nodes-count", 200, "")
	nodeSize := pflag.Float64("node-size", 10, "")
	nodeMinDistance := pflag.Float64("node-min-distance", 15, "")
	colorNodeStr := pflag.String("color-node", "00FF00FF", "")
	colorBgStr := pflag.String("color-background", "00000000", "")
	avgConnsPerNode := pflag.Uint("average-connections-per-node", 5, "")
	domainCount := pflag.Uint("domain-count", 20, "")
	connectionWidth := pflag.Float64("connection-width", 4, "")
	locality := pflag.Float64("locality", 0.5, "")
	pflag.Parse()

	if *locality < 0 {
		*locality = 0
	}
	if *locality > 1 {
		*locality = 1
	}

	colorNode, err := ColorParse(*colorNodeStr)
	if err != nil {
		panic(err)
	}

	colorBg, err := ColorParse(*colorBgStr)
	if err != nil {
		panic(err)
	}

	_ = colorBg

	imgSize := image.Point{
		X: int(*width),
		Y: int(*height),
	}
	nodesPerDomain := make([][]*nodeT, *domainCount+1)
	nodes := make([]*nodeT, 0, *nodesCount)
	for i := 0; i <= int(*nodesCount); i++ {
		for attempt := 0; ; attempt++ {
			if attempt >= 1000 {
				panic("wasn't able to find free space for a node")
			}
			candidate := &nodeT{
				X: rand.Float64() * float64(imgSize.X),
				Y: rand.Float64() * float64(imgSize.Y),
			}
			if canPutNode(candidate, nodes, *nodeMinDistance) {
				nodes = append(nodes, candidate)
				break
			}
		}
	}

	for domainID := 1; domainID <= int(*domainCount); domainID++ {
		x := rand.Float64() * float64(imgSize.X)
		y := rand.Float64() * float64(imgSize.Y)
		r := math.Sqrt(float64(imgSize.X) * float64(imgSize.Y) / float64(*domainCount))
		for _, node := range nodes {
			dist := distance(x, y, node.X, node.Y)
			if dist <= r {
				node.DomainID = uint64(domainID)
				nodesPerDomain[domainID] = append(nodesPerDomain[domainID], node)
			}
		}
	}

	interfaceNodes := make([]*nodeT, 0)
	for _, nodes := range nodesPerDomain {
		if len(nodes) == 0 {
			continue
		}

		interfaceCount := int(float64(len(nodes)) * (1 - *locality))
		if interfaceCount == 0 {
			interfaceCount = 1
		}

		indexes := rand.Perm(len(nodes))
		for i := 0; i < interfaceCount; i++ {
			idx := indexes[i]
			nodes[idx].IsInterface = true
			interfaceNodes = append(interfaceNodes, nodes[idx])
		}
	}

	if len(interfaceNodes) == 0 {
		panic("no interface nodes")
	}

	type connection struct {
		From *nodeT
		To   *nodeT
	}
	connections := map[connection]struct{}{}
	localConnectionsCount := float64(*nodesCount) * float64(*avgConnsPerNode) * *locality
	remoteConnectionsCount := float64(*nodesCount) * float64(*avgConnsPerNode) * (1 - *locality)
	for i := 0; i <= int(remoteConnectionsCount); i++ {
		for {
			idxFrom := rand.Intn(len(interfaceNodes))
			idxTo := rand.Intn(len(interfaceNodes))
			if idxFrom == idxTo {
				continue
			}
			nodeFrom := interfaceNodes[idxFrom]
			nodeTo := interfaceNodes[idxTo]

			if nodeFrom.DomainID == 0 || nodeTo.DomainID == 0 {
				continue
			}
			if nodeFrom.DomainID == nodeTo.DomainID {
				continue
			}

			if _, ok := connections[connection{
				From: nodeFrom,
				To:   nodeTo,
			}]; ok {
				continue
			}
			if _, ok := connections[connection{
				From: nodeTo,
				To:   nodeFrom,
			}]; ok {
				continue
			}

			connections[connection{
				From: nodeFrom,
				To:   nodeTo,
			}] = struct{}{}
			break
		}
	}

	for i := 0; i <= int(localConnectionsCount); i++ {
		for {
			idxFrom := rand.Intn(len(nodes))
			idxTo := rand.Intn(len(nodes))
			if idxFrom == idxTo {
				continue
			}
			nodeFrom := nodes[idxFrom]
			nodeTo := nodes[idxTo]

			if nodeFrom.DomainID == 0 || nodeTo.DomainID == 0 {
				continue
			}
			if nodeFrom.DomainID != nodeTo.DomainID {
				continue
			}

			if _, ok := connections[connection{
				From: nodeFrom,
				To:   nodeTo,
			}]; ok {
				continue
			}
			if _, ok := connections[connection{
				From: nodeTo,
				To:   nodeFrom,
			}]; ok {
				continue
			}

			connections[connection{
				From: nodeFrom,
				To:   nodeTo,
			}] = struct{}{}
			break
		}
	}

	circleSize := image.Point{
		X: int(*nodeSize),
		Y: int(*nodeSize),
	}
	circleSpace := image.Rectangle{
		Min: image.Point{
			X: 0,
			Y: 0,
		},
		Max: circleSize,
	}
	circleMask := image.NewAlpha(circleSpace)
	for y := 0; y < circleSize.Y; y++ {
		for x := 0; x < circleSize.X; x++ {
			x0, y0, r := *nodeSize/2, *nodeSize/2, *nodeSize/2
			dx := float64(x) + 0.5 - x0
			dy := float64(y) + 0.5 - y0
			dist := math.Sqrt(dx*dx + dy*dy)

			// anti-aliasing of a poor man:
			v := r - dist
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			alpha := color.Alpha{uint8(float64(0xff) * v)}
			circleMask.SetAlpha(x, y, alpha)
		}
	}

	img := image.NewRGBA(image.Rectangle{
		Min: image.Point{
			X: 0,
			Y: 0,
		},
		Max: imgSize,
	})
	for conn := range connections {
		f := conn.From
		t := conn.To

		cr := uint32(rand.Intn(256))
		cg := uint32(rand.Intn(256))
		cb := uint32(rand.Intn(256))

		dx := float64(t.X - f.X)
		dy := float64(t.Y - f.Y)
		angle := math.Atan2(dy, dx)

		dist := nodesDistance(f, t)
		for x0, y0, step := float64(f.X), float64(f.Y), 0; step < int(dist); step++ {
			for w := -(*connectionWidth / 2); w < *connectionWidth/2; w++ {
				x := x0 - 0.5 + float64(w)*math.Sin(angle)
				y := y0 - 0.5 + float64(w)*math.Cos(angle)

				// anti-aliasing of a poor man
				xC := int(math.Round(x))
				yC := int(math.Round(y))
				for dx := -1; dx <= 1; dx++ {
					for dy := -1; dy <= 1; dy++ {
						p := image.Point{
							X: xC + dx,
							Y: yC + dy,
						}
						dist := distance(x, y, float64(p.X), float64(p.Y))
						v := 1 - dist
						if v <= 0 {
							continue
						}
						alpha := uint32(float64(0xFF) * v)
						if alpha <= 0 {
							continue
						}
						if alpha > 0xFF {
							alpha = 0xFF
						}
						r, g, b, a := img.At(p.X, p.Y).RGBA()
						a >>= 8
						r = ((r>>8)*a + cr*alpha) / (a + alpha)
						g = ((g>>8)*a + cg*alpha) / (a + alpha)
						b = ((b>>8)*a + cb*alpha) / (a + alpha)
						a = a + alpha
						if a > 0xFF {
							a = 0xFF
						}

						img.Set(p.X, p.Y, color.NRGBA{
							R: uint8(r),
							G: uint8(g),
							B: uint8(b),
							A: uint8(a),
						})
					}
				}
			}
			x0 = float64(x0) + math.Cos(angle)
			y0 = float64(y0) + math.Sin(angle)
		}
	}

	uniformNodeColor := image.NewUniform(colorNode)
	for _, node := range nodes {
		if node.DomainID == 0 {
			continue
		}
		dstBounds := image.Rectangle{
			Min: image.Point{
				X: int(float64(node.X) - *nodeSize/2),
				Y: int(float64(node.Y) - *nodeSize/2),
			},
			Max: image.Point{
				X: int(float64(node.X) + *nodeSize/2),
				Y: int(float64(node.Y) + *nodeSize/2),
			},
		}
		draw.DrawMask(img, dstBounds, uniformNodeColor, image.Point{}, circleMask, image.Point{}, draw.Over)
	}

	err = png.Encode(os.Stdout, img)
	if err != nil {
		panic(err)
	}
}

func canPutNode(new *nodeT, current []*nodeT, minDistance float64) bool {
	// would've been nice to use a k-d tree here, instead of:
	for _, node := range current {
		if nodesDistance(new, node) < minDistance {
			return false
		}
	}
	return true
}

func pointsDistance(a, b image.Point) float64 {
	return distance(float64(a.X), float64(a.Y), float64(b.X), float64(b.Y))
}

func nodesDistance(a, b *nodeT) float64 {
	return distance(a.X, a.Y, b.X, b.Y)
}

func distance(x0, y0, x1, y1 float64) float64 {
	distX := float64(x0 - x1)
	distY := float64(y0 - y1)
	return math.Sqrt(distX*distX + distY*distY)
}
