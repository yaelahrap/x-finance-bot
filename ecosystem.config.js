module.exports = {
  apps: [
    {
      name: "x-finance-bot",
      script: "./x-finance-bot",
      // Pastikan PM2 menjalankan binary dari direktori yang benar
      cwd: "/root/bot/x-finance-bot", 
      // Argumen jika ada (Go binary biasanya tidak butuh kalau pakai .env)
      args: "",
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: "1G",
      env: {
        APP_ENV: "production",
      },
      // Penting: PM2 defaultnya merestart jika file berubah. 
      // Karena SQLite mengubah file di dalam /data, kita abaikan foldernya
      ignore_watch: ["data", "node_modules", ".git"],
      // Tulis log output & error ke file ini
      out_file: "./logs/out.log",
      error_file: "./logs/error.log",
      time: true
    }
  ]
};
