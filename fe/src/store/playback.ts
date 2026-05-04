import { create } from 'zustand';

type PlaybackState = {
  playing: boolean;
  index: number;
  speed: number;
  setPlaying: (playing: boolean) => void;
  restart: () => void;
  step: (delta: number, max: number) => void;
  setSpeed: (speed: number) => void;
};

export const usePlaybackStore = create<PlaybackState>((set) => ({
  playing: false,
  index: 0,
  speed: 1,
  setPlaying: (playing) => set({ playing }),
  restart: () => set({ index: 0, playing: false }),
  step: (delta, max) => set((s) => ({ index: Math.max(0, Math.min(max, s.index + delta)), playing: false })),
  setSpeed: (speed) => set({ speed })
}));

