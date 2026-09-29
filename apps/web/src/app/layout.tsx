import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Open Flow · AI media infrastructure",
  description:
    "A Gemini-first, open-source media gateway. Built for durable workflows and explainable routing.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <a href="#main" className="skip-link">
          Skip to content
        </a>
        {children}
      </body>
    </html>
  );
}
