import { useId } from "react";

import styles from "./Fingerprint.module.css";

export interface FingerprintProps {
  label: string;
  type: string;
  value: string;
  /** The other keys the server presents, smaller. */
  others?: Array<{ type: string; value: string }>;
  othersLabel?: string;
  /** A line under the box, for example where the trusted key is kept. */
  note?: string;
}

/** A server key's fingerprint, large and in a fixed-width face so it can be compared character by character. */
export function Fingerprint({ label, type, value, others = [], othersLabel, note }: FingerprintProps) {
  const id = useId();
  const box = (
    <div role="group" aria-labelledby={id} className={styles.box}>
      <div className={styles.head}>
        <h2 id={id} className={styles.label}>
          {label}
        </h2>
        <span className={styles.type}>{type}</span>
      </div>
      <p className={styles.value}>{value}</p>
      {others.length > 0 && (
        <div className={styles.others}>
          <span>{othersLabel}</span>
          {others.map((o) => (
            <span key={o.value} className={styles.other}>
              {o.type} · {o.value}
            </span>
          ))}
        </div>
      )}
    </div>
  );
  if (!note) return box;
  return (
    <>
      {box}
      <p className={styles.note}>{note}</p>
    </>
  );
}
