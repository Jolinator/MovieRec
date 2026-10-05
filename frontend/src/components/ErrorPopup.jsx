import React from 'react'
import styles from './ErrorPopup.module.css'

export default function ErrorPopup({ message, onClose }) {
  return (
    <div className={styles.overlay} role="alertdialog" aria-modal="true" aria-labelledby="error-title">
      <section className={styles.popup}>
        <h2 id="error-title">Connection error</h2>
        <p>{message}</p>
        <button type="button" onClick={onClose}>Close</button>
      </section>
    </div>
  )
}
