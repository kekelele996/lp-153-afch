import { create } from "zustand";
import type { Capsule } from "@/api/capsule";
import { capsuleApi } from "@/api/capsule";

interface CapsuleState {
  capsules: Capsule[];
  received: Capsule[];
  total: number;
  loading: boolean;
  fetchMine: () => Promise<void>;
  fetchReceived: () => Promise<void>;
}

export const useCapsuleStore = create<CapsuleState>((set) => ({
  capsules: [],
  received: [],
  total: 0,
  loading: false,
  fetchMine: async () => {
    set({ loading: true });
    try {
      const data = await capsuleApi.mine({ page: 1, page_size: 50 });
      set({ capsules: data.items, total: data.total });
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
