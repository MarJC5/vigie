import { useTranslation } from "react-i18next";

import { keyOf } from "../../lib/findings";
import { Switch } from "../Switch/Switch";
import styles from "./LookupTable.module.css";

export interface LookupTableProps {
  /** Each destination a scan may ask (wpvulnerability, wordpress.org, registry, site), on or off. */
  destinations: Array<{ id: string; on: boolean }>;
  onChange: (id: string, on: boolean) => void;
  /** Says under each destination turned off what is not checked, as the settings do. */
  explainOff?: boolean;
}

/** What each service asked during a security scan receives, and how long its answer is kept, with a
 switch per service. IANA comes with the domain registries: it only says which registry to ask. */
export function LookupTable({ destinations, onChange, explainOff = false }: LookupTableProps) {
  const { t } = useTranslation();
  const on = new Map(destinations.map((d) => [d.id, d.on]));
  const rows = destinations.flatMap((d) => (d.id === "registry" ? ["iana", d.id] : [d.id]));
  return (
    <div role="table" aria-label={t("settings.services")} className={styles.table}>
      <div role="row" className={styles.head}>
        <span role="columnheader" className={styles.switchColumn}>
          <span className={styles.hidden}>{t("lookups.colWho")}</span>
        </span>
        <span role="columnheader">{t("lookups.colWho")}</span>
        <span role="columnheader">{t("lookups.colSent")}</span>
        <span role="columnheader" className={styles.kept}>
          {t("lookups.colKept")}
        </span>
      </div>
      {rows.map((id) => {
        const key = `lookups.destination.${keyOf(id)}`;
        const name = t(`${key}.name`);
        // IANA follows the registries.
        const active = id === "iana" ? on.get("registry") : on.get(id);
        return (
          <div role="row" key={id} className={styles.row} data-off={active ? undefined : true}>
            <span role="cell" className={styles.switchColumn}>
              {id !== "iana" && <Switch label={name} on={!!active} onChange={(next) => onChange(id, next)} />}
            </span>
            <span role="cell" className={styles.name}>
              {name}
            </span>
            <span role="cell" className={styles.muted}>
              {t(`${key}.sent`)}
              {explainOff && !active && id !== "iana" && <span className={styles.off}>{t(`lookups.off.${keyOf(id)}`)}</span>}
            </span>
            <span role="cell" className={`${styles.muted} ${styles.kept}`}>
              {t(`${key}.kept`)}
            </span>
          </div>
        );
      })}
    </div>
  );
}
