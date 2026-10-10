import React, { useEffect } from 'react'
import styles from './ConfirmationPopup.module.css'

export default function ConfirmationPopup({ message, onDismiss }) {
  useEffect(() => {
    const timeoutId = window.setTimeout(onDismiss, 3000)
    return () => window.clearTimeout(timeoutId)
  }, [onDismiss])

  return (
    <div className={styles.popup} role="status">
      {message}
    </div>
  )
}
