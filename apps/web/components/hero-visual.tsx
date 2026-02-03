"use client";

import * as React from "react";
import Image from "next/image";
import { motion, useMotionValue, useSpring, useTransform } from "framer-motion";
import { cn } from "@/lib/utils";

type HeroVisualProps = {
  className?: string;
};

export function HeroVisual({ className }: HeroVisualProps) {
  const ref = React.useRef<HTMLDivElement | null>(null);
  const x = useMotionValue(0);
  const y = useMotionValue(0);

  const dampedX = useSpring(x, { stiffness: 90, damping: 18 });
  const dampedY = useSpring(y, { stiffness: 90, damping: 18 });

  const layerBackX = useTransform(dampedX, [-0.5, 0.5], [-14, 14]);
  const layerBackY = useTransform(dampedY, [-0.5, 0.5], [-10, 10]);
  const layerMidX = useTransform(dampedX, [-0.5, 0.5], [-22, 22]);
  const layerMidY = useTransform(dampedY, [-0.5, 0.5], [-16, 16]);
  const layerFrontX = useTransform(dampedX, [-0.5, 0.5], [-30, 30]);
  const layerFrontY = useTransform(dampedY, [-0.5, 0.5], [-20, 20]);

  const handleMove = (event: React.MouseEvent<HTMLDivElement>) => {
    if (!ref.current) return;
    const rect = ref.current.getBoundingClientRect();
    const px = (event.clientX - rect.left) / rect.width - 0.5;
    const py = (event.clientY - rect.top) / rect.height - 0.5;
    x.set(px);
    y.set(py);
  };

  const handleLeave = () => {
    x.set(0);
    y.set(0);
  };

  return (
    <div
      ref={ref}
      onMouseMove={handleMove}
      onMouseLeave={handleLeave}
      className={cn(
        "relative h-[360px] w-full overflow-hidden rounded-[28px] border border-white/10 bg-panel/70 p-6 shadow-soft",
        className
      )}
    >
      <motion.div
        style={{ x: layerBackX, y: layerBackY }}
        className="absolute inset-0"
      >
        <div className="absolute left-[8%] top-[18%] h-40 w-40 rounded-full bg-accent/20 blur-3xl" />
        <div className="absolute bottom-[12%] right-[10%] h-32 w-32 rounded-full bg-accent2/20 blur-3xl" />
      </motion.div>

      <div className="pointer-events-none absolute inset-0 tech-grid opacity-35" />

      <motion.div
        style={{ x: layerMidX, y: layerMidY }}
        className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2"
      >
        <div className="relative rounded-[26px] border border-white/10 bg-panel-2/80 p-5 shadow-glow">
          <div className="pointer-events-none absolute -inset-4 rounded-full border border-accent/30 opacity-70 animate-[spin_18s_linear_infinite]" />
          <div className="pointer-events-none absolute -inset-8 rounded-full border border-accent2/20 opacity-50 animate-[spin_28s_linear_infinite]" />
          <Image
            src="/images/nanobazaar_logo_transparent.png"
            alt="NanoBazaar logo"
            width={160}
            height={160}
            className="relative z-10 h-20 w-20 object-contain sm:h-24 sm:w-24"
            priority
          />
        </div>
      </motion.div>

      <motion.div
        style={{ x: layerMidX, y: layerMidY }}
        className="absolute left-6 top-8 rounded-2xl border border-white/10 bg-white/5 p-4 shadow-soft"
      >
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-ink/60">
          Offer
        </p>
        <p className="mt-2 text-sm font-semibold text-ink">
          Market scan, 12 hours
        </p>
        <p className="mt-2 text-xs text-ink/60">
          Input: topic, region, depth
        </p>
      </motion.div>

      <motion.div
        style={{ x: layerFrontX, y: layerFrontY }}
        className="absolute right-8 top-20 rounded-2xl border border-white/10 bg-gradient-to-br from-white/10 to-white/5 text-ink shadow-soft"
      >
        <div className="px-4 pt-4">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-ink/60">
            Job Accepted
          </p>
          <p className="mt-2 text-sm font-semibold">
            Guidance received
          </p>
        </div>
        <div className="mt-4 border-t border-white/10 px-4 py-3 text-xs text-ink/60">
          Encrypted payloads - Nano settled
        </div>
      </motion.div>

      <motion.div
        style={{ x: layerBackX, y: layerBackY }}
        className="absolute bottom-6 left-10 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-xs text-ink/60 shadow-soft"
      >
        <span className="font-semibold text-ink">Relay</span> never sees plaintext.
      </motion.div>

    </div>
  );
}
