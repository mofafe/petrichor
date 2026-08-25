package voice

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

type playbackMixer struct {
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	mu      sync.Mutex
	sources map[string]*playbackSource
}

type playbackSource struct {
	buffer []int16
	volume float32
}

func newPlaybackMixer() (*playbackMixer, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("init playback context: %w", err)
	}

	mixer := &playbackMixer{
		ctx:     malgoCtx,
		sources: map[string]*playbackSource{},
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = audioChannels
	deviceConfig.SampleRate = audioSampleRate
	deviceConfig.PeriodSizeInMilliseconds = uint32(audioFrameDuration / 1_000_000)
	deviceConfig.Alsa.NoMMap = 1

	device, err := malgo.InitDevice(malgoCtx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(output, _ []byte, frameCount uint32) {
			mixer.fill(output, frameCount)
		},
	})
	if err != nil {
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
		return nil, fmt.Errorf("init playback device: %w", err)
	}
	mixer.device = device
	if err := device.Start(); err != nil {
		device.Uninit()
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
		return nil, fmt.Errorf("start playback device: %w", err)
	}
	return mixer, nil
}

func (m *playbackMixer) Close() {
	if m.device != nil {
		m.device.Uninit()
	}
	if m.ctx != nil {
		_ = m.ctx.Uninit()
		m.ctx.Free()
	}
}

func (m *playbackMixer) AddSource(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sources[id] == nil {
		m.sources[id] = &playbackSource{volume: 1}
	}
}

func (m *playbackMixer) RemoveSource(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sources, id)
}

func (m *playbackMixer) SetVolume(id string, volume float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source := m.sources[id]
	if source == nil {
		source = &playbackSource{}
		m.sources[id] = source
	}
	source.volume = clamp01(volume)
}

func (m *playbackMixer) Push(id string, pcm []int16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source := m.sources[id]
	if source == nil {
		source = &playbackSource{volume: 1}
		m.sources[id] = source
	}
	if len(source.buffer) > audioSamplesPerFrame*12 {
		source.buffer = source.buffer[len(source.buffer)-audioSamplesPerFrame*6:]
	}
	source.buffer = append(source.buffer, pcm...)
}

func (m *playbackMixer) fill(output []byte, frameCount uint32) {
	for i := range output {
		output[i] = 0
	}

	sampleCount := int(frameCount) * audioChannels
	mixed := make([]int32, sampleCount)

	m.mu.Lock()
	for _, source := range m.sources {
		if source.volume <= 0 || len(source.buffer) == 0 {
			continue
		}
		n := sampleCount
		if len(source.buffer) < n {
			n = len(source.buffer)
		}
		for i := 0; i < n; i++ {
			mixed[i] += int32(float32(source.buffer[i]) * source.volume)
		}
		source.buffer = source.buffer[n:]
	}
	m.mu.Unlock()

	for i, sample := range mixed {
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		j := i * 2
		output[j] = byte(int16(sample))
		output[j+1] = byte(int16(sample) >> 8)
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
