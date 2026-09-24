import { create } from "zustand";
import type { Capsule } from "@/api/capsule";
import { capsuleApi } from "@/api/capsule";

interface CapsuleState {
  mine: Capsule[];
  received: Capsule[];
  total: number;
  loading: boolean;
  fetchMine: () => Promise<void>;
  fetchReceived: () => Promise<void>;
}

// 胶囊按「我封存的 / 收到的」两个视角分别缓存。
export const useCapsuleStore = create<CapsuleState>((set) => ({
  mine: [],
  received: [],
  total: 0,
  loading: false,
  fetchMine: async () => {
    set({ loading: true });
    try {
      const data = await capsuleApi.mine({ page: 1, page_size: 50 });
      set({ mine: data.items, total: data.total });
    } finally {
      set({ loading: false });
    }
  },
  fetchReceived: async () => {
    set({ loading: true });
    try {
      const data = await capsuleApi.received({ page: 1, page_size: 50 });
      set({ received: data.items, total: data.total });
    } finally {
      set({ loading: false });
    }
  },
}));
