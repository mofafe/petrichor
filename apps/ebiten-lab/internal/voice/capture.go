package voice

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gen2brain/malgo"
	pionopus "github.com/pion/opus"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const (
	audioSampleRate       = 48000
	audioChannels         = 1
	audioFrameDuration    = 20 * time.Millisecond
	audioSamplesPerFrame  = audioSampleRate / 50
	audioBytesPerSample   = 2
	audioBytesPerFrame    = audioSamplesPerFrame * audioChannels * audioBytesPerSample
	audioEncodeBufferSize = 1275
)

type microphone struct {
	ctx       *malgo.AllocatedContext
	device    *malgo.Device
	cancel    context.CancelFunc
	done      chan struct{}
	frameData chan []byte
}

func startMicrophone(track *webrtc.TrackLocalStaticSample) (*microphone, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("init audio context: %w", err)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = audioChannels
	deviceConfig.SampleRate = audioSampleRate
	deviceConfig.PeriodSizeInMilliseconds = uint32(audioFrameDuration / time.Millisecond)
	deviceConfig.Alsa.NoMMap = 1

	micCtx, cancel := context.WithCancel(context.Background())
	mic := &microphone{
		ctx:       malgoCtx,
		cancel:    cancel,
		done:      make(chan struct{}),
		frameData: make(chan []byte, 8),
	}

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, captured []byte, _ uint32) {
			if len(captured) == 0 {
				return
			}
			copyData := append([]byte(nil), captured...)
			select {
			case mic.frameData <- copyData:
			default:
			}
		},
	}

	device, err := malgo.InitDevice(malgoCtx.Context, deviceConfig, callbacks)
	if err != nil {
		cancel()
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
		return nil, fmt.Errorf("init capture device: %w", err)
	}
	mic.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		cancel()
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
		return nil, fmt.Errorf("start capture device: %w", err)
	}

	go mic.encodeLoop(micCtx, track)
	return mic, nil
}

func (m *microphone) Close() {
	m.cancel()
	<-m.done
	if m.device != nil {
		m.device.Uninit()
	}
	if m.ctx != nil {
		_ = m.ctx.Uninit()
		m.ctx.Free()
	}
}

func (m *microphone) encodeLoop(ctx context.Context, track *webrtc.TrackLocalStaticSample) {
	defer close(m.done)

	encoder, err := pionopus.NewEncoder(
		pionopus.WithSampleRate(audioSampleRate),
		pionopus.WithChannels(audioChannels),
		pionopus.WithApplication(pionopus.ApplicationVoIP),
		pionopus.WithBitrate(24000),
	)
	if err != nil {
		log.Printf("opus encoder init failed: %v", err)
		return
	}

	pcm := make([]byte, 0, audioBytesPerFrame*2)
	packet := make([]byte, audioEncodeBufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-m.frameData:
			pcm = append(pcm, data...)
			for len(pcm) >= audioBytesPerFrame {
				frame := pcm[:audioBytesPerFrame]
				n, err := encoder.Encode(frame, packet)
				if err != nil {
					log.Printf("opus encode failed: %v", err)
					pcm = pcm[audioBytesPerFrame:]
					continue
				}
				if err := track.WriteSample(media.Sample{
					Data:     append([]byte(nil), packet[:n]...),
					Duration: audioFrameDuration,
				}); err != nil {
					log.Printf("write opus sample failed: %v", err)
				}
				pcm = pcm[audioBytesPerFrame:]
			}
		}
	}
}
