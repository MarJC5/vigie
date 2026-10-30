import { useTranslation } from "react-i18next";

import { Icon } from "../Icon/Icon";
import styles from "./AddonImage.module.css";

export interface AddonImageProps {
  /** The add-on's own image, a data URI; empty for Vigie's default one. */
  image: string;
  /** "card" in the grid, "hero" at the top of its page. */
  size?: "card" | "hero";
}

/** An add-on's image, shown as an image (an SVG runs no script there), or the default one. */
export function AddonImage({ image, size = "card" }: AddonImageProps) {
  const { t } = useTranslation();
  if (image) return <img src={image} alt="" className={styles.image} data-size={size} />;
  return (
    <div className={styles.fallback} data-size={size}>
      <Icon name="puzzle" size={size === "hero" ? 48 : 40} />
      <span className={styles.label}>{t("addons.defaultImage")}</span>
    </div>
  );
}
