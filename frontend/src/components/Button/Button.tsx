import type { ButtonHTMLAttributes, ReactNode } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./Button.module.css";

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  /** primary: the main action; danger: a main action that removes; outline: secondary; icon: icon
   only. An action that reads as a link is a LinkButton. */
  variant?: "primary" | "danger" | "outline" | "icon";
  icon?: IconName;
  trailingIcon?: IconName;
  /** Accessible name; required for the icon variant, which shows no text. */
  label?: string;
  children?: ReactNode;
}

export function Button({ variant = "outline", icon, trailingIcon, label, children, className, ...rest }: ButtonProps) {
  // A button waiting for an answer turns its icon into a spinner.
  const busy = rest["aria-busy"] === true || rest["aria-busy"] === "true";
  return (
    <button
      type="button"
      {...rest}
      aria-label={label}
      className={[styles.button, styles[variant], className].filter(Boolean).join(" ")}
    >
      {busy ? (
        <span className={styles.spinner}>
          <Icon name="spinner" />
        </span>
      ) : (
        icon && <Icon name={icon} />
      )}
      {children}
      {trailingIcon && <Icon name={trailingIcon} size="sm" />}
    </button>
  );
}
