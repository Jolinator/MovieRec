import React from 'react'
import styles from './LogoutBanner.module.css'

export default function LogoutBanner({ onCancel, onConfirm }) {
  return (
    <div className={styles.overlay} role="alertdialog" aria-modal="true" aria-labelledby="logout-title">
      <section className={styles.banner}>
        <h2 id="logout-title">Log out?</h2>
        <p>Are you sure you want to log out?</p>
        <div className={styles.actions}>
          <button type="button" className={styles.cancel} onClick={onCancel}>
            Cancel
          </button>
          <button type="button" className={styles.confirm} onClick={onConfirm}>
            Log out
          </button>
        </div>
      </section>
    </div>
  )
}
