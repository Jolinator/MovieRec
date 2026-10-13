import React from 'react'
import styles from './UserBadge.module.css'

export default function UserBadge({ username }) {
  return <p className={styles.badge}>{username || 'Not signed in'}</p>
}
