import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import { Icon } from "../Icon/Icon";
import { LookupTable } from "../LookupTable/LookupTable";
import { Notice } from "../Notice/Notice";
import styles from "./ConsentScreen.module.css";

export interface ConsentScreenProps {
  /** The destinations a scan may ask, every one on at first. */
  destinations: string[];
  /** Agrees, with the destinations left on. */
  onAgree: (on: string[]) => void;
  onNotNow: () => void;
}

/** Before the first security scan: what each service receives, a switch per service, and nothing
 sent until the user agrees. */
export function ConsentScreen({ destinations, onAgree, onNotNow }: ConsentScreenProps) {
  const { t } = useTranslation();
  const [on, setOn] = useState(() => new Set(destinations));
  return (
    <div className={styles.window}>
      <div className={styles.top} />
      <main className={styles.main} tabIndex={0}>
        <div className={styles.content}>
          <div className={styles.heading}>
            <span className={styles.icon}>
              <Icon name="shield" size="xl" />
            </span>
            <h1 className={styles.title}>{t("consent.title")}</h1>
            <p className={styles.lead}>{t("consent.lead")}</p>
          </div>
          <LookupTable
            destinations={destinations.map((id) => ({ id, on: on.has(id) }))}
            onChange={(id, next) => {
              const changed = new Set(on);
              if (next) changed.add(id);
              else changed.delete(id);
              setOn(changed);
            }}
          />
          <Notice tone="neutral" icon="lock" role="note" title={t("lookups.neverTitle")} text={t("lookups.neverText")} />
        </div>
      </main>
      <div className={styles.bar}>
        <span className={styles.later}>{t("consent.later")}</span>
        <span className={styles.actions}>
          <Button onClick={onNotNow}>{t("consent.notNow")}</Button>
          <Button variant="primary" onClick={() => onAgree(destinations.filter((d) => on.has(d)))}>
            {t("consent.agree")}
          </Button>
        </span>
      </div>
    </div>
  );
}
