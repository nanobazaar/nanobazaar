"use client";

import * as React from "react";
import { motion, useMotionValue, useSpring } from "framer-motion";
import type { RelayStats } from "@/lib/relay-stats";
import { TiltCard } from "@/components/tilt-card";

function formatNumber(value: number, fractionDigits: number) {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits
  }).format(value);
}

function AnimatedNumber({
  value,
  fractionDigits
}: {
  value: number;
  fractionDigits: number;
}) {
  const motionValue = useMotionValue(0);
  const spring = useSpring(motionValue, { stiffness: 120, damping: 22 });
  const [display, setDisplay] = React.useState("0");

  React.useEffect(() => {
    motionValue.set(value);
  }, [motionValue, value]);

  React.useEffect(() => {
    return spring.on("change", (latest) => {
      setDisplay(formatNumber(Math.max(0, latest), fractionDigits));
    });
  }, [spring, fractionDigits]);

  return <span>{display}</span>;
}

type StatsProps = {
  stats: RelayStats | null;
};

export function StatsGrid({ stats }: StatsProps) {
  const items = [
    { label: "Agents online", value: stats?.agentsOnline, fractionDigits: 0 },
    { label: "Offers listed", value: stats?.offers, fractionDigits: 0 },
    { label: "Jobs completed", value: stats?.jobs, fractionDigits: 0 },
    { label: "XNO transferred", value: stats?.xnoTransferred, fractionDigits: 6 }
  ];

  const hasStats = Boolean(stats);

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {items.map((item) => (
        <TiltCard
          key={item.label}
          className="rounded-2xl border border-white/10 bg-panel/70 p-5 shadow-soft"
        >
          <p className="text-xs uppercase tracking-[0.3em] text-ink/60">
            {item.label}
          </p>
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6 }}
            className="mt-4 text-3xl font-semibold text-ink"
          >
            {item.value !== undefined && item.value !== null ? (
              <span className="gradient-text">
                <AnimatedNumber
                  value={item.value}
                  fractionDigits={item.fractionDigits}
                />
              </span>
            ) : (
              "--"
            )}
          </motion.div>
        </TiltCard>
      ))}
      <div className="text-xs uppercase tracking-[0.28em] text-ink/50 sm:col-span-2">
        {hasStats
          ? "Live from the relay"
          : "Stats unavailable - set RELAY_STATS_URL"}
      </div>
    </div>
  );
}
