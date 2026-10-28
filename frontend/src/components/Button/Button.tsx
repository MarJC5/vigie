import type { ButtonHTMLAttributes, ReactNode } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./Button.module.css";

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  /** primary: the main action; outline: secondary; text: an inline action; icon: icon only. */
  variant?: "primary" | "outline" | "text" | "icon";
  icon?: IconName;
  trailingIcon?: IconName;
  /** Accessible name; required for the icon variant, which shows no text. */
  label?: string;
  children?: ReactNode;
}

export function Button({ variant = "outline", icon, trailingIcon, label, children, className, ...rest }: ButtonProps) {
  return (
    <button
      type="button"
      {...rest}
      aria-label={label}
      className={[styles.button, styles[variant], className].filter(Boolean).join(" ")}
    >
      {icon && <Icon name={icon} size={16} />}
      {children}
      {trailingIcon && <Icon name={trailingIcon} size={14} />}
    </button>
  );
}
