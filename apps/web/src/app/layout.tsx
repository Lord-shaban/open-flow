import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Open Flow · Your creative space",
  description:
    "Your ideas, brought to life with free image providers. A private creative workspace powered by Kafka and Temporal.",
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
